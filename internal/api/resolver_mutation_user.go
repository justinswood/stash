package api

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"golang.org/x/crypto/bcrypt"
)

// enabledAdminCount counts non-disabled admin accounts. Used to prevent removing
// the last way to administer the system.
func enabledAdminCount(users []*models.User) int {
	n := 0
	for _, u := range users {
		if u.Role == models.UserRoleAdmin && !u.Disabled {
			n++
		}
	}
	return n
}

func (r *mutationResolver) UserCreate(ctx context.Context, input UserCreateInput) (*models.User, error) {
	if err := r.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if input.Username == "" || input.Password == "" {
		return nil, errors.New("username and password are required")
	}

	hash, err := manager.HashUserPassword(input.Password)
	if err != nil {
		return nil, err
	}

	u := &models.User{
		Username:     input.Username,
		PasswordHash: hash,
		Role:         input.Role,
	}
	if input.Disabled != nil {
		u.Disabled = *input.Disabled
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		existing, err := r.repository.User.FindByUsername(ctx, input.Username)
		if err != nil {
			return err
		}
		if existing != nil {
			return fmt.Errorf("username %q already exists", input.Username)
		}
		return r.repository.User.Create(ctx, u)
	}); err != nil {
		return nil, err
	}
	refreshDisabledUsers(ctx)
	return u, nil
}

// refreshDisabledUsers reloads the in-memory disabled-account set after a user
// mutation, so a disable takes effect on the next request instead of when the
// account's session cookie eventually expires. Best effort: the GraphQL path
// re-checks against the database anyway (getCurrentUser), so a failure here
// degrades to the pre-existing behaviour on media routes rather than granting
// anything.
func refreshDisabledUsers(ctx context.Context) {
	if err := manager.GetInstance().RefreshDisabledUsers(ctx); err != nil {
		logger.Errorf("error refreshing disabled user accounts: %v", err)
	}
}

func (r *mutationResolver) UserUpdate(ctx context.Context, input UserUpdateInput) (*models.User, error) {
	if err := r.requireAdmin(ctx); err != nil {
		return nil, err
	}
	id, err := strconv.Atoi(input.ID)
	if err != nil {
		return nil, err
	}

	var ret *models.User
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		u, err := r.repository.User.Find(ctx, id)
		if err != nil {
			return err
		}
		if u == nil {
			return fmt.Errorf("user %d not found", id)
		}

		// capture whether this change removes the last enabled admin
		wasEnabledAdmin := u.Role == models.UserRoleAdmin && !u.Disabled

		if input.Username != nil {
			u.Username = *input.Username
		}
		if input.Role != nil {
			u.Role = *input.Role
		}
		if input.Disabled != nil {
			u.Disabled = *input.Disabled
		}
		if input.Password != nil && *input.Password != "" {
			hash, err := manager.HashUserPassword(*input.Password)
			if err != nil {
				return err
			}
			u.PasswordHash = hash
		}

		nowEnabledAdmin := u.Role == models.UserRoleAdmin && !u.Disabled
		if wasEnabledAdmin && !nowEnabledAdmin {
			all, err := r.repository.User.All(ctx)
			if err != nil {
				return err
			}
			if enabledAdminCount(all) <= 1 {
				return errors.New("cannot demote or disable the last enabled admin")
			}
		}

		if err := r.repository.User.Update(ctx, u); err != nil {
			return err
		}
		ret = u
		return nil
	}); err != nil {
		return nil, err
	}
	refreshDisabledUsers(ctx)
	return ret, nil
}

func (r *mutationResolver) UserDestroy(ctx context.Context, id string) (bool, error) {
	if err := r.requireAdmin(ctx); err != nil {
		return false, err
	}
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return false, err
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		u, err := r.repository.User.Find(ctx, idInt)
		if err != nil {
			return err
		}
		if u == nil {
			return nil
		}
		if u.Role == models.UserRoleAdmin && !u.Disabled {
			all, err := r.repository.User.All(ctx)
			if err != nil {
				return err
			}
			if enabledAdminCount(all) <= 1 {
				return errors.New("cannot delete the last enabled admin")
			}
		}
		return r.repository.User.Destroy(ctx, idInt)
	}); err != nil {
		return false, err
	}
	refreshDisabledUsers(ctx)
	return true, nil
}

func (r *mutationResolver) ChangePassword(ctx context.Context, input ChangePasswordInput) (bool, error) {
	current, err := r.getCurrentUser(ctx)
	if err != nil {
		return false, err
	}
	if current == nil {
		return false, ErrPermission
	}
	if input.NewPassword == "" {
		return false, errors.New("new password is required")
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		u, err := r.repository.User.FindByUsername(ctx, current.Username)
		if err != nil {
			return err
		}
		if u == nil {
			return errors.New("current user has no account record")
		}

		// verify current password
		if input.CurrentPassword == nil ||
			bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(*input.CurrentPassword)) != nil {
			return errors.New("current password is incorrect")
		}

		hash, err := manager.HashUserPassword(input.NewPassword)
		if err != nil {
			return err
		}
		u.PasswordHash = hash
		return r.repository.User.Update(ctx, u)
	}); err != nil {
		return false, err
	}
	return true, nil
}
