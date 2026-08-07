//go:build integration
// +build integration

package api

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sqlite"
	"github.com/stashapp/stash/pkg/txn"
)

// A real database is required here rather than a mocked one. The thing under
// test is whether each media route's *Ctx middleware consults the content
// visibility layer at all, and that layer runs SQL. Against a mock the
// visibility call fails for want of a database, the middleware treats the
// object as hidden, and the test reports 404 — the expected answer, arrived at
// for the wrong reason, and it would keep passing with the guard deleted.

var (
	db *sqlite.Database

	// performers
	performerHiddenGenderID int
	performerHiddenTagID    int
	performerVisibleID      int

	// content attached to the above
	sceneHiddenID    int
	sceneVisibleID   int
	imageHiddenID    int
	imageVisibleID   int
	galleryHiddenID  int
	galleryVisibleID int

	// the tag named by the restriction
	excludedTagID int
)

// excludedGender is the gender the test restrictions hide.
const excludedGender = models.GenderEnumTransgenderFemale

// testRestrictions is the restriction set applied by the scoping tests. It
// mirrors the shape of a real user group: some genders, some tags.
func testRestrictions() models.ContentRestrictions {
	return models.ContentRestrictions{
		ExcludedGenders: []string{excludedGender.String()},
		ExcludedTagIDs:  []int{excludedTagID},
	}
}

func TestMain(m *testing.M) {
	// some migrations read config
	_ = config.InitializeEmpty()

	os.Exit(runScopingTests(m))
}

func runScopingTests(m *testing.M) int {
	f, err := os.CreateTemp("", "*.sqlite")
	if err != nil {
		panic(fmt.Sprintf("could not create temporary database file: %v", err))
	}
	f.Close()
	databaseFile := f.Name()

	db = sqlite.NewDatabase()
	db.SetBlobStoreOptions(sqlite.BlobStoreOptions{
		// keep blobs in the database so the test needs no blob directory
		UseDatabase: true,
	})

	if err := db.Open(databaseFile); err != nil {
		panic(fmt.Sprintf("could not initialise database: %v", err))
	}

	defer func() {
		if err := db.Close(); err != nil {
			panic(err)
		}
		if err := os.Remove(databaseFile); err != nil {
			panic(err)
		}
	}()

	if err := populateScopingDB(); err != nil {
		panic(fmt.Sprintf("could not populate database: %v", err))
	}

	return m.Run()
}

func withTxn(f func(ctx context.Context) error) error {
	return txn.WithTxn(context.Background(), db, f)
}

// populateScopingDB builds the smallest library that can distinguish "hidden"
// from "visible" on every restricted type: one performer excluded by gender,
// one excluded by tag, one visible, and a scene/image/gallery attached to each
// of the hidden and visible performers.
func populateScopingDB() error {
	return withTxn(func(ctx context.Context) error {
		tag := models.Tag{Name: "scoping-excluded-tag"}
		if err := db.Tag.Create(ctx, &tag); err != nil {
			return fmt.Errorf("creating tag: %w", err)
		}
		excludedTagID = tag.ID

		hiddenGender := excludedGender
		visibleGender := models.GenderEnumFemale

		hiddenByGender := models.Performer{
			Name:   "scoping hidden by gender",
			Gender: &hiddenGender,
		}
		if err := db.Performer.Create(ctx, &models.CreatePerformerInput{Performer: &hiddenByGender}); err != nil {
			return fmt.Errorf("creating gender-hidden performer: %w", err)
		}
		performerHiddenGenderID = hiddenByGender.ID

		hiddenByTag := models.Performer{
			Name:   "scoping hidden by tag",
			Gender: &visibleGender,
			TagIDs: models.NewRelatedIDs([]int{excludedTagID}),
		}
		if err := db.Performer.Create(ctx, &models.CreatePerformerInput{Performer: &hiddenByTag}); err != nil {
			return fmt.Errorf("creating tag-hidden performer: %w", err)
		}
		performerHiddenTagID = hiddenByTag.ID

		visible := models.Performer{
			Name:   "scoping visible",
			Gender: &visibleGender,
		}
		if err := db.Performer.Create(ctx, &models.CreatePerformerInput{Performer: &visible}); err != nil {
			return fmt.Errorf("creating visible performer: %w", err)
		}
		performerVisibleID = visible.ID

		// Content is hidden by association: a scene featuring an excluded
		// performer is itself excluded.
		mk := func(hiddenPerformer, visiblePerformer int) error {
			hiddenScene := models.Scene{
				Title:        "scoping hidden scene",
				PerformerIDs: models.NewRelatedIDs([]int{hiddenPerformer}),
			}
			if err := db.Scene.Create(ctx, &hiddenScene, nil); err != nil {
				return fmt.Errorf("creating hidden scene: %w", err)
			}
			sceneHiddenID = hiddenScene.ID

			visibleScene := models.Scene{
				Title:        "scoping visible scene",
				PerformerIDs: models.NewRelatedIDs([]int{visiblePerformer}),
			}
			if err := db.Scene.Create(ctx, &visibleScene, nil); err != nil {
				return fmt.Errorf("creating visible scene: %w", err)
			}
			sceneVisibleID = visibleScene.ID

			hiddenImage := models.Image{
				Title:        "scoping hidden image",
				PerformerIDs: models.NewRelatedIDs([]int{hiddenPerformer}),
			}
			if err := db.Image.Create(ctx, &hiddenImage, nil); err != nil {
				return fmt.Errorf("creating hidden image: %w", err)
			}
			imageHiddenID = hiddenImage.ID

			visibleImage := models.Image{
				Title:        "scoping visible image",
				PerformerIDs: models.NewRelatedIDs([]int{visiblePerformer}),
			}
			if err := db.Image.Create(ctx, &visibleImage, nil); err != nil {
				return fmt.Errorf("creating visible image: %w", err)
			}
			imageVisibleID = visibleImage.ID

			hiddenGallery := models.Gallery{
				Title:        "scoping hidden gallery",
				PerformerIDs: models.NewRelatedIDs([]int{hiddenPerformer}),
			}
			if err := db.Gallery.Create(ctx, &hiddenGallery, nil); err != nil {
				return fmt.Errorf("creating hidden gallery: %w", err)
			}
			galleryHiddenID = hiddenGallery.ID

			visibleGallery := models.Gallery{
				Title:        "scoping visible gallery",
				PerformerIDs: models.NewRelatedIDs([]int{visiblePerformer}),
			}
			if err := db.Gallery.Create(ctx, &visibleGallery, nil); err != nil {
				return fmt.Errorf("creating visible gallery: %w", err)
			}
			galleryVisibleID = visibleGallery.ID

			return nil
		}

		return mk(performerHiddenGenderID, performerVisibleID)
	})
}
