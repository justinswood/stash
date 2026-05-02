package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/stashapp/stash/pkg/models"
)

const shareLinkTokenBytes = 32

// generateShareToken creates a cryptographically random URL-safe token
// and returns (plaintext, sha256-hash).
func generateShareToken() (string, []byte, error) {
	buf := make([]byte, shareLinkTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("generating token: %w", err)
	}
	plaintext := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(plaintext))
	return plaintext, sum[:], nil
}

// HashShareToken hashes a plaintext share token for DB lookup.
func HashShareToken(plaintext string) []byte {
	sum := sha256.Sum256([]byte(plaintext))
	return sum[:]
}

func parseIDPtr(s *string) (*int, error) {
	if s == nil {
		return nil, nil
	}
	v, err := strconv.Atoi(*s)
	if err != nil {
		return nil, fmt.Errorf("invalid id %q: %w", *s, err)
	}
	return &v, nil
}

func (r *mutationResolver) ShareLinkCreate(ctx context.Context, input ShareLinkCreateInput) (*models.ShareLink, error) {
	if !input.ShareType.IsValid() {
		return nil, fmt.Errorf("invalid share_type")
	}

	sceneID, err := parseIDPtr(input.SceneID)
	if err != nil {
		return nil, err
	}
	imageID, err := parseIDPtr(input.ImageID)
	if err != nil {
		return nil, err
	}
	performerID, err := parseIDPtr(input.PerformerID)
	if err != nil {
		return nil, err
	}

	// Validate that the right ID is set for the share type.
	switch input.ShareType {
	case models.ShareTypeScene:
		if sceneID == nil {
			return nil, errors.New("scene_id is required when share_type is SCENE")
		}
	case models.ShareTypeImage:
		if imageID == nil {
			return nil, errors.New("image_id is required when share_type is IMAGE")
		}
	case models.ShareTypePerformer:
		if performerID == nil {
			return nil, errors.New("performer_id is required when share_type is PERFORMER")
		}
	}

	plaintext, tokenHash, err := generateShareToken()
	if err != nil {
		return nil, err
	}

	link := &models.ShareLink{
		TokenHash:   tokenHash,
		ShareType:   input.ShareType,
		SceneID:     sceneID,
		ImageID:     imageID,
		PerformerID: performerID,
		ExpiresAt:   nil,
		ViewLimit:   nil,
		Note:        input.Note,
	}
	if input.ExpiresAt != nil {
		t := *input.ExpiresAt
		link.ExpiresAt = &t
	}
	if input.ViewLimit != nil {
		v := *input.ViewLimit
		link.ViewLimit = &v
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		return r.repository.ShareLink.Create(ctx, link)
	}); err != nil {
		return nil, err
	}

	// Stash the plaintext token + URL on the response object via the ctx-scoped map
	// so the field resolvers can return them. We use a context key to pass these
	// "transient" values from the mutation to the field resolvers.
	stashShareLinkPlaintext(ctx, link.ID, plaintext)
	return link, nil
}

func (r *mutationResolver) ShareLinkUpdate(ctx context.Context, input ShareLinkUpdateInput) (*models.ShareLink, error) {
	id, err := strconv.Atoi(input.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid id: %w", err)
	}

	var ret *models.ShareLink
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		existing, err := r.repository.ShareLink.Find(ctx, id)
		if err != nil {
			return err
		}
		if existing == nil {
			return fmt.Errorf("share link %d not found", id)
		}
		if input.ExpiresAt != nil {
			t := *input.ExpiresAt
			existing.ExpiresAt = &t
		}
		if input.ViewLimit != nil {
			v := *input.ViewLimit
			existing.ViewLimit = &v
		}
		if input.Revoked != nil {
			existing.Revoked = *input.Revoked
		}
		if input.Note != nil {
			existing.Note = input.Note
		}
		if err := r.repository.ShareLink.Update(ctx, existing); err != nil {
			return err
		}
		ret = existing
		return nil
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *mutationResolver) ShareLinkRevoke(ctx context.Context, id string) (*models.ShareLink, error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid id: %w", err)
	}
	var ret *models.ShareLink
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		existing, err := r.repository.ShareLink.Find(ctx, idInt)
		if err != nil {
			return err
		}
		if existing == nil {
			return fmt.Errorf("share link %d not found", idInt)
		}
		existing.Revoked = true
		if err := r.repository.ShareLink.Update(ctx, existing); err != nil {
			return err
		}
		ret = existing
		return nil
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *mutationResolver) ShareLinkDestroy(ctx context.Context, id string) (bool, error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return false, fmt.Errorf("invalid id: %w", err)
	}
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		return r.repository.ShareLink.Destroy(ctx, idInt)
	}); err != nil {
		return false, err
	}
	return true, nil
}

// IsShareLinkExpired returns true if the link has an expiry timestamp in the past.
func IsShareLinkExpired(link *models.ShareLink) bool {
	if link.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*link.ExpiresAt)
}
