package manager

import (
	"context"

	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"golang.org/x/crypto/bcrypt"
)

// HashUserPassword returns a bcrypt hash of the given plaintext password.
func HashUserPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// EnsureBootstrapAdmin creates the initial admin account from the config
// credential when the users table is empty. Idempotent — safe to call on every
// startup once the database schema is current.
func (s *Manager) EnsureBootstrapAdmin(ctx context.Context) error {
	if s.Database == nil || s.Database.Ready() != nil {
		// schema not yet at the required version (migration pending)
		return nil
	}
	if !s.Config.HasCredentials() {
		// no config credential to seed from; system stays open until an admin
		// account is created
		return nil
	}

	return s.Repository.WithTxn(ctx, func(ctx context.Context) error {
		count, err := s.Repository.User.Count(ctx)
		if err != nil {
			return err
		}
		if count > 0 {
			return nil
		}

		username, pwHash := s.Config.GetCredentials()
		if username == "" || pwHash == "" {
			return nil
		}

		u := &models.User{
			Username:     username,
			PasswordHash: pwHash,
			Role:         models.UserRoleAdmin,
		}
		if err := s.Repository.User.Create(ctx, u); err != nil {
			return err
		}
		logger.Infof("Created bootstrap admin account %q from config credential", username)
		return nil
	})
}

// ValidateUserCredentials checks a username/password against the users table.
// found reports whether an (enabled) account with that username exists; valid
// reports whether the password matched. Wired into the session store so login
// authenticates against user accounts, falling back to the config credential.
func (s *Manager) ValidateUserCredentials(username, password string) (found bool, valid bool) {
	ctx := context.Background()
	if s.Database == nil || s.Database.Ready() != nil {
		return false, false
	}

	err := s.Repository.WithReadTxn(ctx, func(ctx context.Context) error {
		u, err := s.Repository.User.FindByUsername(ctx, username)
		if err != nil {
			return err
		}
		if u == nil || u.Disabled {
			// treat a disabled account as "not found" so no config fallback
			// silently re-enables it; disabled users simply cannot log in
			found = u != nil && u.Disabled
			return nil
		}
		found = true
		valid = bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil
		return nil
	})
	if err != nil {
		logger.Errorf("error validating user credentials: %v", err)
		return false, false
	}
	return found, valid
}

// GetUserByUsername returns the user account for a username, or nil if none.
func (s *Manager) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	if s.Database == nil || s.Database.Ready() != nil {
		// database not open yet (e.g. migration pending) — avoid a nil-DB txn
		return nil, nil
	}
	var ret *models.User
	err := s.Repository.WithReadTxn(ctx, func(ctx context.Context) error {
		u, err := s.Repository.User.FindByUsername(ctx, username)
		ret = u
		return err
	})
	return ret, err
}
