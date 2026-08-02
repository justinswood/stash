package api

import (
	"context"
	"strconv"

	"github.com/stashapp/stash/pkg/models"
)

// resolveTargetUserID determines which account an API key operation applies to.
// A nil/empty userID means "the current user". Targeting anyone else requires
// admin — without this check any USER could mint a key for an admin account and
// escalate.
func (r *Resolver) resolveTargetUserID(ctx context.Context, userID *string) (int, error) {
	current, err := r.getCurrentUser(ctx)
	if err != nil {
		return 0, err
	}
	if current == nil {
		return 0, ErrPermission
	}

	if userID == nil || *userID == "" {
		if current.ID == 0 {
			// synthetic principal (DB not ready); it owns no key rows
			return 0, ErrPermission
		}
		return current.ID, nil
	}

	target, err := strconv.Atoi(*userID)
	if err != nil {
		return 0, err
	}
	if target != current.ID {
		if err := r.requireCap(ctx, models.CapManageUsers); err != nil {
			return 0, err
		}
	}
	return target, nil
}

func (r *queryResolver) FindUserAPIKeys(ctx context.Context, userID *string) (ret []*models.UserAPIKey, err error) {
	target, err := r.resolveTargetUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.UserAPIKey.FindByUserID(ctx, target)
		return err
	}); err != nil {
		return nil, err
	}
	if ret == nil {
		ret = []*models.UserAPIKey{}
	}
	return ret, nil
}
