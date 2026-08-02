package api

import (
	"context"
	"strconv"

	"github.com/stashapp/stash/pkg/models"
)

func (r *queryResolver) Me(ctx context.Context) (*models.User, error) {
	return r.getCurrentUser(ctx)
}

// AllCapabilities lists every capability the schema defines, so a permissions
// UI can render the full set without hardcoding it client-side.
func (r *queryResolver) AllCapabilities(ctx context.Context) ([]models.Capability, error) {
	return models.AllCapabilities, nil
}

func (r *queryResolver) FindUsers(ctx context.Context) (ret []*models.User, err error) {
	if err := r.requireCap(ctx, models.CapManageUsers); err != nil {
		return nil, err
	}
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.User.All(ctx)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *queryResolver) FindUser(ctx context.Context, id string) (ret *models.User, err error) {
	if err := r.requireCap(ctx, models.CapManageUsers); err != nil {
		return nil, err
	}
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.User.Find(ctx, idInt)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}
