package models

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"
)

// UserRole controls what a user is permitted to do. Roles are hierarchical in
// capability: Admin > User > ReadOnly.
type UserRole string

const (
	// UserRoleAdmin can do everything: settings, library tasks, user management,
	// and destructive operations.
	UserRoleAdmin UserRole = "ADMIN"
	// UserRoleUser can browse and edit metadata and manage their own history,
	// but cannot access settings, run library tasks, delete content, or manage
	// users.
	UserRoleUser UserRole = "USER"
	// UserRoleReadOnly can browse and record their own view history only.
	UserRoleReadOnly UserRole = "READ_ONLY"
)

func (e UserRole) IsValid() bool {
	switch e {
	case UserRoleAdmin, UserRoleUser, UserRoleReadOnly:
		return true
	}
	return false
}

func (e UserRole) String() string {
	return string(e)
}

// AtLeast reports whether this role has at least the capability level of other.
func (e UserRole) AtLeast(other UserRole) bool {
	return roleRank(e) >= roleRank(other)
}

func roleRank(r UserRole) int {
	switch r {
	case UserRoleAdmin:
		return 3
	case UserRoleUser:
		return 2
	case UserRoleReadOnly:
		return 1
	}
	return 0
}

func (e *UserRole) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}
	*e = UserRole(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid UserRole", str)
	}
	return nil
}

func (e UserRole) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

// User is a local account with its own login and role. PasswordHash is never
// exposed through the API.
type User struct {
	ID           int       `db:"id" json:"id"`
	Username     string    `db:"username" json:"username"`
	PasswordHash string    `db:"password_hash" json:"-"`
	Role         UserRole  `db:"role" json:"role"`
	Disabled     bool      `db:"disabled" json:"disabled"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

type UserReader interface {
	Find(ctx context.Context, id int) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	All(ctx context.Context) ([]*User, error)
	Count(ctx context.Context) (int, error)
}

type UserWriter interface {
	Create(ctx context.Context, obj *User) error
	Update(ctx context.Context, obj *User) error
	Destroy(ctx context.Context, id int) error
}

type UserReaderWriter interface {
	UserReader
	UserWriter
}
