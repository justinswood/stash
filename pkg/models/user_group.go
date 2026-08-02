package models

import (
	"context"
	"time"
)

// UserGroup names a set of content that its members must not see.
//
// Restrictions across groups are unioned, so membership can only ever hide
// more. That is deliberate: a "restricted" group must not be weakened by also
// belonging to a broader one.
type UserGroup struct {
	ID          int       `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// ContentRestrictions is the resolved set of things a request must not see.
// Empty means unrestricted, which is the case for every account that belongs
// to no group.
type ContentRestrictions struct {
	// ExcludedGenders hides performers of these genders, and any scene, image
	// or gallery featuring one.
	ExcludedGenders []string
	// ExcludedTagIDs hides content carrying these tags, and performers carrying
	// them.
	ExcludedTagIDs []int
}

func (r ContentRestrictions) Empty() bool {
	return len(r.ExcludedGenders) == 0 && len(r.ExcludedTagIDs) == 0
}

type UserGroupReader interface {
	Find(ctx context.Context, id int) (*UserGroup, error)
	FindByName(ctx context.Context, name string) (*UserGroup, error)
	All(ctx context.Context) ([]*UserGroup, error)
	// MemberIDs returns the user ids belonging to a group.
	MemberIDs(ctx context.Context, groupID int) ([]int, error)
	// GroupIDsForUser returns the groups a user belongs to.
	GroupIDsForUser(ctx context.Context, userID int) ([]int, error)
	ExcludedGenders(ctx context.Context, groupID int) ([]string, error)
	ExcludedTagIDs(ctx context.Context, groupID int) ([]int, error)
	// RestrictionsForUser resolves the union of a user's groups' exclusions.
	RestrictionsForUser(ctx context.Context, userID int) (ContentRestrictions, error)
}

type UserGroupWriter interface {
	Create(ctx context.Context, obj *UserGroup) error
	Update(ctx context.Context, obj *UserGroup) error
	Destroy(ctx context.Context, id int) error
	SetMembers(ctx context.Context, groupID int, userIDs []int) error
	SetExcludedGenders(ctx context.Context, groupID int, genders []string) error
	SetExcludedTagIDs(ctx context.Context, groupID int, tagIDs []int) error
}

type UserGroupReaderWriter interface {
	UserGroupReader
	UserGroupWriter
}
