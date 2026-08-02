package models

import (
	"context"
	"time"
)

// UserAPIKey is a named, revocable API key belonging to a single user account.
// The plaintext key is never stored — only KeyHash — so it cannot be recovered
// after creation. Requests authenticating with the key act as the owning user
// and inherit that user's role, which is what makes a key revocable by
// disabling or deleting its owner.
type UserAPIKey struct {
	ID     int    `db:"id" json:"id"`
	UserID int    `db:"user_id" json:"user_id"`
	Name   string `db:"name" json:"name"`
	// KeyHash is the SHA-256 of the plaintext key. Never exposed through the API.
	KeyHash []byte `db:"key_hash" json:"-"`
	// Prefix is the leading plaintext characters, stored so the UI can
	// distinguish keys in a list. It is not secret and not sufficient to
	// authenticate.
	Prefix     string     `db:"prefix" json:"prefix"`
	LastUsedAt *time.Time `db:"last_used_at" json:"last_used_at"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at" json:"updated_at"`
}

type UserAPIKeyReader interface {
	Find(ctx context.Context, id int) (*UserAPIKey, error)
	FindByKeyHash(ctx context.Context, keyHash []byte) (*UserAPIKey, error)
	FindByUserID(ctx context.Context, userID int) ([]*UserAPIKey, error)
}

type UserAPIKeyWriter interface {
	Create(ctx context.Context, obj *UserAPIKey) error
	Destroy(ctx context.Context, id int) error
	// TouchLastUsed records that a key was just used to authenticate. Best
	// effort: a failure here must never block the request.
	TouchLastUsed(ctx context.Context, id int, when time.Time) error
}

type UserAPIKeyReaderWriter interface {
	UserAPIKeyReader
	UserAPIKeyWriter
}
