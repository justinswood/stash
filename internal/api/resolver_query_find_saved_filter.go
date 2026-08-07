package api

import (
	"context"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/utils"
)

func (r *queryResolver) FindSavedFilter(ctx context.Context, id string) (ret *models.SavedFilter, err error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}

	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.SavedFilter.Find(ctx, idInt)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, err
}

func (r *queryResolver) FindSavedFilters(ctx context.Context, mode *models.FilterMode) (ret []*models.SavedFilter, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		if mode != nil {
			ret, err = r.repository.SavedFilter.FindByMode(ctx, *mode)
		} else {
			ret, err = r.repository.SavedFilter.All(ctx)
		}
		return err
	}); err != nil {
		return nil, err
	}
	return ret, err
}

func (r *queryResolver) FindDefaultFilter(ctx context.Context, mode models.FilterMode) (ret *models.SavedFilter, err error) {
	// deprecated - read from the UI configuration in the meantime.
	// That configuration is per-account since migration 85, so a default filter
	// set by one user no longer decides what every other account sees when it
	// opens a list page.
	uiConfig, err := r.userUIConfig(ctx)
	if err != nil {
		return nil, err
	}
	if len(uiConfig) == 0 {
		return nil, nil
	}

	m := utils.NestedMap(uiConfig)
	filterRaw, _ := m.Get("defaultFilters." + strings.ToLower(mode.String()))

	if filterRaw == nil {
		return nil, nil
	}

	ret = &models.SavedFilter{}
	d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:          "json",
		WeaklyTypedInput: true,
		Result:           ret,
	})

	if err != nil {
		return nil, err
	}

	if err := d.Decode(filterRaw); err != nil {
		return nil, err
	}

	return ret, nil
}
