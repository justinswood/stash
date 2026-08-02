package api

import (
	"context"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/models"
)

// Capabilities resolves a user's effective permission set: role preset plus
// grants, minus revokes. Exposed so the UI can show what an account can
// actually do rather than making the client re-derive it from the role.
func (r *userResolver) Capabilities(ctx context.Context, obj *models.User) ([]models.Capability, error) {
	// A synthetic principal (database not open) has no row, so no overrides.
	if obj.ID == 0 {
		return models.CapabilitiesForRole(obj.Role).Sorted(), nil
	}

	overrides, err := manager.GetInstance().GetUserCapabilityOverrides(ctx, obj.ID)
	if err != nil {
		return nil, err
	}
	return models.EffectiveCapabilities(obj.Role, obj.Disabled, overrides).Sorted(), nil
}

// CapabilityOverrides resolves only the departures from the role preset.
func (r *userResolver) CapabilityOverrides(ctx context.Context, obj *models.User) ([]*models.UserCapabilityOverride, error) {
	ret := []*models.UserCapabilityOverride{}
	if obj.ID == 0 {
		return ret, nil
	}

	overrides, err := manager.GetInstance().GetUserCapabilityOverrides(ctx, obj.ID)
	if err != nil {
		return nil, err
	}
	models.SortOverrides(overrides)
	for i := range overrides {
		ret = append(ret, &overrides[i])
	}
	return ret, nil
}
