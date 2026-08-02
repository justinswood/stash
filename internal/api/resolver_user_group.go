package api

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

func atoiSlice(in []string) ([]int, error) {
	ret := make([]int, 0, len(in))
	for _, s := range in {
		v, err := strconv.Atoi(s)
		if err != nil {
			return nil, err
		}
		ret = append(ret, v)
	}
	return ret, nil
}

// --- queries ---

func (r *queryResolver) FindUserGroups(ctx context.Context) (ret []*models.UserGroup, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.UserGroup.All(ctx)
		return err
	}); err != nil {
		return nil, err
	}
	if ret == nil {
		ret = []*models.UserGroup{}
	}
	return ret, nil
}

func (r *queryResolver) FindUserGroup(ctx context.Context, id string) (ret *models.UserGroup, err error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.UserGroup.Find(ctx, idInt)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

// --- field resolvers ---

func (r *userGroupResolver) Users(ctx context.Context, obj *models.UserGroup) (ret []*models.User, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ids, err := r.repository.UserGroup.MemberIDs(ctx, obj.ID)
		if err != nil {
			return err
		}
		for _, id := range ids {
			u, err := r.repository.User.Find(ctx, id)
			if err != nil {
				return err
			}
			if u != nil {
				ret = append(ret, u)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if ret == nil {
		ret = []*models.User{}
	}
	return ret, nil
}

func (r *userGroupResolver) ExcludedGenders(ctx context.Context, obj *models.UserGroup) ([]models.GenderEnum, error) {
	var ret []models.GenderEnum
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		vals, err := r.repository.UserGroup.ExcludedGenders(ctx, obj.ID)
		if err != nil {
			return err
		}
		for _, v := range vals {
			g := models.GenderEnum(v)
			if g.IsValid() {
				ret = append(ret, g)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if ret == nil {
		ret = []models.GenderEnum{}
	}
	return ret, nil
}

func (r *userGroupResolver) ExcludedTags(ctx context.Context, obj *models.UserGroup) (ret []*models.Tag, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ids, err := r.repository.UserGroup.ExcludedTagIDs(ctx, obj.ID)
		if err != nil {
			return err
		}
		for _, id := range ids {
			t, err := r.repository.Tag.Find(ctx, id)
			if err != nil {
				return err
			}
			if t != nil {
				ret = append(ret, t)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if ret == nil {
		ret = []*models.Tag{}
	}
	return ret, nil
}

// --- mutations ---

func (r *mutationResolver) UserGroupCreate(ctx context.Context, input UserGroupCreateInput) (*models.UserGroup, error) {
	if input.Name == "" {
		return nil, errors.New("name is required")
	}

	g := &models.UserGroup{Name: input.Name}
	if input.Description != nil {
		g.Description = *input.Description
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		existing, err := r.repository.UserGroup.FindByName(ctx, input.Name)
		if err != nil {
			return err
		}
		if existing != nil {
			return fmt.Errorf("a group named %q already exists", input.Name)
		}
		if err := r.repository.UserGroup.Create(ctx, g); err != nil {
			return err
		}
		return r.applyUserGroupSets(ctx, g.ID, input.UserIds, input.ExcludedGenders, input.ExcludedTagIds)
	}); err != nil {
		return nil, err
	}
	refreshContentRestrictions(ctx)
	return g, nil
}

// refreshContentRestrictions reloads the cached per-account restrictions after
// a group change, so membership takes effect on the next request rather than
// at the next restart.
func refreshContentRestrictions(ctx context.Context) {
	if err := manager.GetInstance().RefreshContentRestrictions(ctx); err != nil {
		logger.Errorf("error refreshing content restrictions: %v", err)
	}
}

func (r *mutationResolver) UserGroupUpdate(ctx context.Context, input UserGroupUpdateInput) (*models.UserGroup, error) {
	id, err := strconv.Atoi(input.ID)
	if err != nil {
		return nil, err
	}

	var ret *models.UserGroup
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		g, err := r.repository.UserGroup.Find(ctx, id)
		if err != nil {
			return err
		}
		if g == nil {
			return fmt.Errorf("group %d not found", id)
		}

		if input.Name != nil {
			g.Name = *input.Name
		}
		if input.Description != nil {
			g.Description = *input.Description
		}
		if err := r.repository.UserGroup.Update(ctx, g); err != nil {
			return err
		}

		if err := r.applyUserGroupSets(ctx, id, input.UserIds, input.ExcludedGenders, input.ExcludedTagIds); err != nil {
			return err
		}
		ret = g
		return nil
	}); err != nil {
		return nil, err
	}
	refreshContentRestrictions(ctx)
	return ret, nil
}

// applyUserGroupSets writes the membership and exclusion lists. Each is
// replace-entirely, and a nil slice means "leave alone" — distinguishing that
// from an empty slice is what lets an update change only the name.
func (r *Resolver) applyUserGroupSets(
	ctx context.Context,
	groupID int,
	userIDs []string,
	genders []models.GenderEnum,
	tagIDs []string,
) error {
	if userIDs != nil {
		ids, err := atoiSlice(userIDs)
		if err != nil {
			return err
		}
		if err := r.repository.UserGroup.SetMembers(ctx, groupID, ids); err != nil {
			return err
		}
	}
	if genders != nil {
		vals := make([]string, 0, len(genders))
		for _, g := range genders {
			if !g.IsValid() {
				return fmt.Errorf("unknown gender %q", g)
			}
			vals = append(vals, g.String())
		}
		if err := r.repository.UserGroup.SetExcludedGenders(ctx, groupID, vals); err != nil {
			return err
		}
	}
	if tagIDs != nil {
		ids, err := atoiSlice(tagIDs)
		if err != nil {
			return err
		}
		if err := r.repository.UserGroup.SetExcludedTagIDs(ctx, groupID, ids); err != nil {
			return err
		}
	}
	return nil
}

func (r *mutationResolver) UserGroupDestroy(ctx context.Context, id string) (bool, error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return false, err
	}
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		return r.repository.UserGroup.Destroy(ctx, idInt)
	}); err != nil {
		return false, err
	}
	refreshContentRestrictions(ctx)
	return true, nil
}
