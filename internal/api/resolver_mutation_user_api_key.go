package api

import (
	"context"
	"errors"
	"strconv"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/models"
)

// UserAPIKeyCreated carries the one-and-only-time plaintext key alongside the
// stored record. Autobound by gqlgen from this package.
type UserAPIKeyCreated struct {
	APIKey *models.UserAPIKey `json:"api_key"`
	Key    string             `json:"key"`
}

func (r *mutationResolver) UserAPIKeyCreate(ctx context.Context, input UserAPIKeyCreateInput) (*UserAPIKeyCreated, error) {
	target, err := r.resolveTargetUserID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	if input.Name == "" {
		return nil, errors.New("name is required")
	}

	// confirm the target account exists before minting a key for it
	var targetExists bool
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		u, err := r.repository.User.Find(ctx, target)
		targetExists = u != nil
		return err
	}); err != nil {
		return nil, err
	}
	if !targetExists {
		return nil, errors.New("user not found")
	}

	k, plaintext, err := manager.GetInstance().CreateUserAPIKey(ctx, target, input.Name)
	if err != nil {
		return nil, err
	}

	return &UserAPIKeyCreated{APIKey: k, Key: plaintext}, nil
}

func (r *mutationResolver) UserAPIKeyRevoke(ctx context.Context, id string) (bool, error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return false, err
	}

	current, err := r.getCurrentUser(ctx)
	if err != nil {
		return false, err
	}
	if current == nil {
		return false, ErrPermission
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		k, err := r.repository.UserAPIKey.Find(ctx, idInt)
		if err != nil {
			return err
		}
		if k == nil {
			return errors.New("api key not found")
		}
		// owners revoke their own keys; anything else requires admin
		if k.UserID != current.ID {
			if err := r.requireCap(ctx, models.CapManageUsers); err != nil {
				return err
			}
		}
		return r.repository.UserAPIKey.Destroy(ctx, idInt)
	}); err != nil {
		return false, err
	}
	return true, nil
}
