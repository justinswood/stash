package api

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/stashapp/stash/pkg/models"
)

// UserCapabilitiesSet replaces an account's override set.
//
// An override that matches what the role preset already says is dropped rather
// than stored: keeping it would silently pin the capability if the account's
// role later changed, which is not what "grant X" means to whoever set it.
func (r *mutationResolver) UserCapabilitiesSet(ctx context.Context, input UserCapabilitiesSetInput) (*models.User, error) {
	if err := r.requireCap(ctx, models.CapManageUsers); err != nil {
		return nil, err
	}

	id, err := strconv.Atoi(input.UserID)
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

		preset := models.CapabilitiesForRole(u.Role)

		seen := make(map[models.Capability]bool, len(input.Overrides))
		overrides := make([]models.UserCapabilityOverride, 0, len(input.Overrides))
		for _, o := range input.Overrides {
			if !o.Capability.IsValid() {
				return fmt.Errorf("unknown capability %q", o.Capability)
			}
			if seen[o.Capability] {
				return fmt.Errorf("duplicate override for %q", o.Capability)
			}
			seen[o.Capability] = true

			// redundant with the role preset — storing it would outlive a role change
			if preset.Has(o.Capability) == o.Granted {
				continue
			}
			overrides = append(overrides, models.UserCapabilityOverride{
				UserID:     id,
				Capability: o.Capability,
				Granted:    o.Granted,
			})
		}

		// Refuse to strip the last account that can manage users. Mirrors the
		// last-enabled-admin guard on userUpdate/userDestroy: without it an admin
		// could revoke MANAGE_USERS from themselves and lock everyone out of user
		// administration with no way back short of editing the database.
		effective := models.EffectiveCapabilities(u.Role, u.Disabled, overrides)
		if !effective.Has(models.CapManageUsers) {
			others, err := r.otherUserManagerCount(ctx, id)
			if err != nil {
				return err
			}
			if others == 0 {
				return errors.New("cannot remove MANAGE_USERS from the last account that has it")
			}
		}

		if err := r.repository.UserCapability.SetForUser(ctx, id, overrides); err != nil {
			return err
		}
		ret = u
		return nil
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

// otherUserManagerCount counts enabled accounts other than excludeID whose
// effective capabilities include MANAGE_USERS.
func (r *Resolver) otherUserManagerCount(ctx context.Context, excludeID int) (int, error) {
	all, err := r.repository.User.All(ctx)
	if err != nil {
		return 0, err
	}

	n := 0
	for _, u := range all {
		if u.ID == excludeID || u.Disabled {
			continue
		}
		overrides, err := r.repository.UserCapability.FindByUserID(ctx, u.ID)
		if err != nil {
			return 0, err
		}
		if models.EffectiveCapabilities(u.Role, u.Disabled, overrides).Has(models.CapManageUsers) {
			n++
		}
	}
	return n, nil
}
