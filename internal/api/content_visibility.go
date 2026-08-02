package api

import (
	"context"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sqlite"
)

// The query-builder filters in pkg/sqlite scope list queries. Fetching an
// object by id goes through Find, which those filters never touch — so without
// these guards a restricted account could reach hidden content by navigating
// straight to /scenes/4.
//
// Each returns nil rather than an error: from the caller's point of view the
// object does not exist, and saying "forbidden" would confirm that it does.

func (r *queryResolver) hideRestrictedScene(ctx context.Context, s *models.Scene) (*models.Scene, error) {
	if s == nil || !sqlite.RestrictionsActive(ctx) {
		return s, nil
	}
	var visible bool
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		visible, err = sqlite.SceneVisible(ctx, s.ID)
		return err
	}); err != nil {
		return nil, err
	}
	if !visible {
		return nil, nil
	}
	return s, nil
}

func (r *queryResolver) hideRestrictedPerformer(ctx context.Context, p *models.Performer) (*models.Performer, error) {
	if p == nil || !sqlite.RestrictionsActive(ctx) {
		return p, nil
	}
	var visible bool
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		visible, err = sqlite.PerformerVisible(ctx, p.ID)
		return err
	}); err != nil {
		return nil, err
	}
	if !visible {
		return nil, nil
	}
	return p, nil
}

func (r *queryResolver) hideRestrictedImage(ctx context.Context, i *models.Image) (*models.Image, error) {
	if i == nil || !sqlite.RestrictionsActive(ctx) {
		return i, nil
	}
	var visible bool
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		visible, err = sqlite.ImageVisible(ctx, i.ID)
		return err
	}); err != nil {
		return nil, err
	}
	if !visible {
		return nil, nil
	}
	return i, nil
}

func (r *queryResolver) hideRestrictedGallery(ctx context.Context, g *models.Gallery) (*models.Gallery, error) {
	if g == nil || !sqlite.RestrictionsActive(ctx) {
		return g, nil
	}
	var visible bool
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		visible, err = sqlite.GalleryVisible(ctx, g.ID)
		return err
	}); err != nil {
		return nil, err
	}
	if !visible {
		return nil, nil
	}
	return g, nil
}
