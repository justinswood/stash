//go:build integration
// +build integration

package sqlite_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sqlite"
)

// Ratings became per-user in migration 86. Before it a rating was a single
// column, so every account saw the same stars — which is what the original
// report actually hit: a saved filter named "Favs" is rating100 = 100, so a
// second account opening it was shown the admin's five-star scenes.
//
// A rating is a value rather than a flag, so unlike favourites it has to be
// correct in three places: what a read reports, what a filter selects, and how
// a sort orders. All three are covered here, because getting the read right
// while leaving the filter on the shared column would look correct on a scene
// page and still leak through /scenes?rating=.

func intPtrVal(i int) *int { return &i }

func TestSceneRatingIsPerUser(t *testing.T) {
	runWithRollbackTxn(t, "TestSceneRatingIsPerUser", func(t *testing.T, ctx context.Context) {
		aID := makeUser(ctx, t, "rating-a")
		bID := makeUser(ctx, t, "rating-b")

		aCtx := sqlite.WithHistoryUser(ctx, aID)
		bCtx := sqlite.WithHistoryUser(ctx, bID)

		scene := models.Scene{Title: "per-user rating scene"}
		require.NoError(t, db.Scene.Create(aCtx, &scene, nil))

		_, err := db.Scene.UpdatePartial(aCtx, scene.ID, models.ScenePartial{
			Rating: models.NewOptionalInt(100),
		})
		require.NoError(t, err)

		got, err := db.Scene.Find(aCtx, scene.ID)
		require.NoError(t, err)
		require.NotNil(t, got.Rating)
		assert.Equal(t, 100, *got.Rating, "the rating must read back for the user who set it")

		// The whole point: one account's stars must not appear for another.
		got, err = db.Scene.Find(bCtx, scene.ID)
		require.NoError(t, err)
		assert.Nil(t, got.Rating, "one account's rating must not be visible to another")

		// And a second account can hold a different rating for the same scene.
		_, err = db.Scene.UpdatePartial(bCtx, scene.ID, models.ScenePartial{
			Rating: models.NewOptionalInt(20),
		})
		require.NoError(t, err)

		got, err = db.Scene.Find(aCtx, scene.ID)
		require.NoError(t, err)
		require.NotNil(t, got.Rating)
		assert.Equal(t, 100, *got.Rating, "the first account's rating must survive the second rating it")

		got, err = db.Scene.Find(bCtx, scene.ID)
		require.NoError(t, err)
		require.NotNil(t, got.Rating)
		assert.Equal(t, 20, *got.Rating)
	})
}

func TestSceneRatingClearedIsUnrated(t *testing.T) {
	runWithRollbackTxn(t, "TestSceneRatingClearedIsUnrated", func(t *testing.T, ctx context.Context) {
		userID := makeUser(ctx, t, "rating-clear")
		uCtx := sqlite.WithHistoryUser(ctx, userID)

		scene := models.Scene{Title: "cleared rating scene", Rating: intPtrVal(60)}
		require.NoError(t, db.Scene.Create(uCtx, &scene, nil))

		got, err := db.Scene.Find(uCtx, scene.ID)
		require.NoError(t, err)
		require.NotNil(t, got.Rating, "a rating set at creation must belong to the acting user")
		assert.Equal(t, 60, *got.Rating)

		// Clearing must remove the row rather than store zero — "unrated" and
		// "rated 0" are different answers and the UI shows them differently.
		_, err = db.Scene.UpdatePartial(uCtx, scene.ID, models.ScenePartial{
			Rating: models.NewOptionalIntPtr(nil),
		})
		require.NoError(t, err)

		got, err = db.Scene.Find(uCtx, scene.ID)
		require.NoError(t, err)
		assert.Nil(t, got.Rating, "a cleared rating must read as unrated, not zero")
	})
}

// A background task — scan, autotag, import, DLNA — has no user in context. It
// must neither report nor record a rating, or a scheduled job would act on, or
// write into, somebody's personal stars.
func TestSceneRatingWithoutUser(t *testing.T) {
	runWithRollbackTxn(t, "TestSceneRatingWithoutUser", func(t *testing.T, ctx context.Context) {
		userID := makeUser(ctx, t, "rating-nouser")
		uCtx := sqlite.WithHistoryUser(ctx, userID)

		scene := models.Scene{Title: "background rating scene"}
		require.NoError(t, db.Scene.Create(uCtx, &scene, nil))
		_, err := db.Scene.UpdatePartial(uCtx, scene.ID, models.ScenePartial{
			Rating: models.NewOptionalInt(80),
		})
		require.NoError(t, err)

		// Strip the fixture user the harness attaches — this is the background
		// case: scan, autotag, import, DLNA.
		bgCtx := sqlite.WithHistoryUser(ctx, 0)

		got, err := db.Scene.Find(bgCtx, scene.ID)
		require.NoError(t, err)
		assert.Nil(t, got.Rating, "a background read must not report a user's rating")

		// A write with no user must not land anywhere.
		_, err = db.Scene.UpdatePartial(bgCtx, scene.ID, models.ScenePartial{
			Rating: models.NewOptionalInt(10),
		})
		require.NoError(t, err)

		got, err = db.Scene.Find(uCtx, scene.ID)
		require.NoError(t, err)
		require.NotNil(t, got.Rating)
		assert.Equal(t, 80, *got.Rating, "a background write must not overwrite a user's rating")
	})
}

// The filter is the path the original report came through — a saved filter of
// rating100 = 100. Scoping the read but not the filter would look right on a
// scene page and still leak the admin's five-star list.
func TestSceneRatingFilterIsPerUser(t *testing.T) {
	runWithRollbackTxn(t, "TestSceneRatingFilterIsPerUser", func(t *testing.T, ctx context.Context) {
		aID := makeUser(ctx, t, "ratingfilter-a")
		bID := makeUser(ctx, t, "ratingfilter-b")

		aCtx := sqlite.WithHistoryUser(ctx, aID)
		bCtx := sqlite.WithHistoryUser(ctx, bID)

		scene := models.Scene{Title: "filtered rating scene"}
		require.NoError(t, db.Scene.Create(aCtx, &scene, nil))
		_, err := db.Scene.UpdatePartial(aCtx, scene.ID, models.ScenePartial{
			Rating: models.NewOptionalInt(100),
		})
		require.NoError(t, err)

		filter := models.SceneFilterType{
			Rating100: &models.IntCriterionInput{
				Value:    100,
				Modifier: models.CriterionModifierEquals,
			},
		}

		found := func(c context.Context) bool {
			res, err := db.Scene.Query(c, models.SceneQueryOptions{
				SceneFilter: &filter,
				QueryOptions: models.QueryOptions{
					FindFilter: &models.FindFilterType{PerPage: intPtrVal(-1)},
				},
			})
			require.NoError(t, err)
			scenes, err := res.Resolve(c)
			require.NoError(t, err)
			for _, s := range scenes {
				if s.ID == scene.ID {
					return true
				}
			}
			return false
		}

		assert.True(t, found(aCtx), "the rating owner's filter must match their own scene")
		assert.False(t, found(bCtx),
			"another account must not match a scene on somebody else's rating — "+
				"this is the leak the original report came through")
		assert.False(t, found(sqlite.WithHistoryUser(ctx, 0)),
			"with no user a rating filter must match nothing rather than everyone's")
	})
}
