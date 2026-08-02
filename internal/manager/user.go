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

// RefreshDisabledUsers reloads the in-memory set of disabled accounts. Call it
// at startup and after any mutation that could change an account's disabled
// state (create, update, delete), so a disable takes effect on the very next
// request rather than whenever the user's session cookie happens to expire.
func (s *Manager) RefreshDisabledUsers(ctx context.Context) error {
	if s.Database == nil || s.Database.Ready() != nil {
		return nil
	}

	set := make(map[string]struct{})
	if err := s.Repository.WithReadTxn(ctx, func(ctx context.Context) error {
		users, err := s.Repository.User.All(ctx)
		if err != nil {
			return err
		}
		for _, u := range users {
			if u.Disabled {
				set[u.Username] = struct{}{}
			}
		}
		return nil
	}); err != nil {
		return err
	}

	s.disabledUsersMu.Lock()
	s.disabledUsers = set
	s.disabledUsersMu.Unlock()
	return nil
}

// IsUserDisabled reports whether the named account is disabled. Backed by an
// in-memory set so it is safe to call on every request, including media routes
// where a per-asset database lookup would be too expensive.
//
// Authoritative for non-GraphQL routes. The GraphQL path additionally re-checks
// against the database in getCurrentUser, so a stale cache there cannot grant
// access.
func (s *Manager) IsUserDisabled(username string) bool {
	if username == "" {
		return false
	}
	s.disabledUsersMu.RLock()
	defer s.disabledUsersMu.RUnlock()
	_, disabled := s.disabledUsers[username]
	return disabled
}

// GetUserCapabilityOverrides returns a user's per-account departures from their
// role preset. An empty result is the normal case — presets cover most accounts.
func (s *Manager) GetUserCapabilityOverrides(ctx context.Context, userID int) ([]models.UserCapabilityOverride, error) {
	if s.Database == nil || s.Database.Ready() != nil {
		return nil, nil
	}
	var ret []models.UserCapabilityOverride
	err := s.Repository.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		ret, err = s.Repository.UserCapability.FindByUserID(ctx, userID)
		return err
	})
	return ret, err
}

// RefreshContentRestrictions reloads every account's resolved restrictions.
// Call after any change to groups, group membership, or the accounts
// themselves, so a restriction takes effect on the next request.
func (s *Manager) RefreshContentRestrictions(ctx context.Context) error {
	if s.Database == nil || s.Database.Ready() != nil {
		return nil
	}

	set := make(map[string]models.ContentRestrictions)
	if err := s.Repository.WithReadTxn(ctx, func(ctx context.Context) error {
		users, err := s.Repository.User.All(ctx)
		if err != nil {
			return err
		}
		for _, u := range users {
			r, err := s.Repository.UserGroup.RestrictionsForUser(ctx, u.ID)
			if err != nil {
				return err
			}
			if !r.Empty() {
				set[u.Username] = r
			}
		}
		return nil
	}); err != nil {
		return err
	}

	s.userRestrictionsMu.Lock()
	s.userRestrictions = set
	s.userRestrictionsMu.Unlock()
	return nil
}

// ContentRestrictionsForUsername returns cached restrictions for an account.
// Safe to call on every request; empty for accounts in no group.
func (s *Manager) ContentRestrictionsForUsername(username string) models.ContentRestrictions {
	if username == "" {
		return models.ContentRestrictions{}
	}
	s.userRestrictionsMu.RLock()
	defer s.userRestrictionsMu.RUnlock()
	return s.userRestrictions[username]
}

// GetContentRestrictionsForUser resolves what a user must not see, from the
// union of their group memberships. Empty for accounts in no group, which is
// every account until a group is created and populated.
func (s *Manager) GetContentRestrictionsForUser(ctx context.Context, userID int) (models.ContentRestrictions, error) {
	var ret models.ContentRestrictions
	if s.Database == nil || s.Database.Ready() != nil {
		return ret, nil
	}
	err := s.Repository.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		ret, err = s.Repository.UserGroup.RestrictionsForUser(ctx, userID)
		return err
	})
	return ret, err
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
