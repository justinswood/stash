package api

import (
	"context"
	"strconv"

	"github.com/stashapp/stash/pkg/models"
)

// Ratings are per-user (migration 86). These mutations exist so that setting
// one does not require EDIT_METADATA.
//
// A rating used to ride on the object's update mutation — sceneUpdate and
// friends — which is correct for shared library metadata but wrong once the
// rating belongs to the caller alone: it left a READ_ONLY account unable to
// rate anything, while the thing it was being denied changes nothing anybody
// else can see. permissions_middleware routes every *SetRating field to
// CapOwnViewSettings, which all three role presets hold.
//
// Each returns the updated object rather than a boolean so that Apollo
// normalises the new rating into its cache on the way back, without every
// call site having to write a cache update by hand.

func ratingPartial(rating100 *int) models.OptionalInt {
	if rating100 == nil {
		return models.NewOptionalIntPtr(nil)
	}
	return models.NewOptionalInt(*rating100)
}

func (r *mutationResolver) SceneSetRating(ctx context.Context, id string, rating100 *int) (*models.Scene, error) {
	sceneID, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}

	var ret *models.Scene
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Scene.UpdatePartial(ctx, sceneID, models.ScenePartial{
			Rating: ratingPartial(rating100),
		})
		return err
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *mutationResolver) PerformerSetRating(ctx context.Context, id string, rating100 *int) (*models.Performer, error) {
	performerID, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}

	var ret *models.Performer
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Performer.UpdatePartial(ctx, performerID, models.PerformerPartial{
			Rating: ratingPartial(rating100),
		})
		return err
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *mutationResolver) GallerySetRating(ctx context.Context, id string, rating100 *int) (*models.Gallery, error) {
	galleryID, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}

	var ret *models.Gallery
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Gallery.UpdatePartial(ctx, galleryID, models.GalleryPartial{
			Rating: ratingPartial(rating100),
		})
		return err
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *mutationResolver) ImageSetRating(ctx context.Context, id string, rating100 *int) (*models.Image, error) {
	imageID, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}

	var ret *models.Image
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Image.UpdatePartial(ctx, imageID, models.ImagePartial{
			Rating: ratingPartial(rating100),
		})
		return err
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *mutationResolver) GroupSetRating(ctx context.Context, id string, rating100 *int) (*models.Group, error) {
	groupID, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}

	var ret *models.Group
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Group.UpdatePartial(ctx, groupID, models.GroupPartial{
			Rating: ratingPartial(rating100),
		})
		return err
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *mutationResolver) StudioSetRating(ctx context.Context, id string, rating100 *int) (*models.Studio, error) {
	studioID, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}

	var ret *models.Studio
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		// StudioPartial carries its own id, unlike the other partials
		ret, err = r.repository.Studio.UpdatePartial(ctx, models.StudioPartial{
			ID:     studioID,
			Rating: ratingPartial(rating100),
		})
		return err
	}); err != nil {
		return nil, err
	}

	return ret, nil
}
