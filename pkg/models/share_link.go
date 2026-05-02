package models

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"
)

type ShareType string

const (
	ShareTypeScene     ShareType = "SCENE"
	ShareTypeImage     ShareType = "IMAGE"
	ShareTypePerformer ShareType = "PERFORMER"
)

func (e ShareType) IsValid() bool {
	switch e {
	case ShareTypeScene, ShareTypeImage, ShareTypePerformer:
		return true
	}
	return false
}

func (e ShareType) String() string {
	return string(e)
}

func (e *ShareType) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}
	*e = ShareType(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ShareType", str)
	}
	return nil
}

func (e ShareType) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

// ShareLink represents a tokenized link granting read-only access to specific
// content without authenticating to Stash itself.
type ShareLink struct {
	ID          int        `db:"id" json:"id"`
	TokenHash   []byte     `db:"token_hash" json:"-"`
	ShareType   ShareType  `db:"share_type" json:"share_type"`
	SceneID     *int       `db:"scene_id" json:"scene_id"`
	ImageID     *int       `db:"image_id" json:"image_id"`
	PerformerID *int       `db:"performer_id" json:"performer_id"`
	ExpiresAt   *time.Time `db:"expires_at" json:"expires_at"`
	ViewLimit   *int       `db:"view_limit" json:"view_limit"`
	ViewCount   int        `db:"view_count" json:"view_count"`
	Revoked     bool       `db:"revoked" json:"revoked"`
	Note        *string    `db:"note" json:"note"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
}

type ShareLinkReader interface {
	Find(ctx context.Context, id int) (*ShareLink, error)
	FindByTokenHash(ctx context.Context, tokenHash []byte) (*ShareLink, error)
	All(ctx context.Context) ([]*ShareLink, error)
}

type ShareLinkWriter interface {
	Create(ctx context.Context, obj *ShareLink) error
	Update(ctx context.Context, obj *ShareLink) error
	Destroy(ctx context.Context, id int) error
	IncrementViewCount(ctx context.Context, id int) (int, error)
}

type ShareLinkReaderWriter interface {
	ShareLinkReader
	ShareLinkWriter
}
