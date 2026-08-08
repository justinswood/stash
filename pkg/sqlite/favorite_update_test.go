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

// Favourites are per-user (migration 83). Three write paths can set one —
// Create, UpdatePartial and the full-object Update — and all three must agree,
// because a path that quietly skips the join table loses the favourite with no
// error anywhere. Update was the path that skipped it.

func TestPerformerUpdateCarriesFavouriteForActingUser(t *testing.T) {
	runWithRollbackTxn(t, "TestPerformerUpdateCarriesFavouriteForActingUser", func(t *testing.T, ctx context.Context) {
		ctx = sqlite.WithHistoryUser(ctx, testUserID)

		p := models.Performer{Name: "favourite via update"}
		require.NoError(t, db.Performer.Create(ctx, &models.CreatePerformerInput{Performer: &p}))
		require.False(t, p.Favorite, "a performer created without a favourite must not have one")

		p.Favorite = true
		require.NoError(t, db.Performer.Update(ctx, &models.UpdatePerformerInput{Performer: &p}))

		got, err := db.Performer.Find(ctx, p.ID)
		require.NoError(t, err)
		assert.True(t, got.Favorite,
			"a full-object update must record the favourite for the acting user, "+
				"as Create and UpdatePartial already do")

		// And it must be removable the same way, or a user could never unfavourite
		// through this path.
		p.Favorite = false
		require.NoError(t, db.Performer.Update(ctx, &models.UpdatePerformerInput{Performer: &p}))

		got, err = db.Performer.Find(ctx, p.ID)
		require.NoError(t, err)
		assert.False(t, got.Favorite, "clearing the favourite must remove it for the acting user")
	})
}

func TestPerformerUpdateFavouriteIsPerUser(t *testing.T) {
	runWithRollbackTxn(t, "TestPerformerUpdateFavouriteIsPerUser", func(t *testing.T, ctx context.Context) {
		other := models.User{
			Username:     "other-user",
			PasswordHash: "not-a-real-hash",
			Role:         models.UserRoleUser,
		}
		require.NoError(t, db.User.Create(ctx, &other))

		actingCtx := sqlite.WithHistoryUser(ctx, testUserID)
		otherCtx := sqlite.WithHistoryUser(ctx, other.ID)

		p := models.Performer{Name: "favourite belongs to one user"}
		require.NoError(t, db.Performer.Create(actingCtx, &models.CreatePerformerInput{Performer: &p}))

		p.Favorite = true
		require.NoError(t, db.Performer.Update(actingCtx, &models.UpdatePerformerInput{Performer: &p}))

		// The whole point of migration 83: one account favouriting something
		// must not favourite it for everybody, which is what the old shared
		// column did.
		got, err := db.Performer.Find(otherCtx, p.ID)
		require.NoError(t, err)
		assert.False(t, got.Favorite,
			"one user's favourite must not appear for another account")

		got, err = db.Performer.Find(actingCtx, p.ID)
		require.NoError(t, err)
		assert.True(t, got.Favorite, "the acting user must still see their own favourite")
	})
}

func TestPerformerUpdateWithoutUserDoesNotFavourite(t *testing.T) {
	runWithRollbackTxn(t, "TestPerformerUpdateWithoutUserDoesNotFavourite", func(t *testing.T, ctx context.Context) {
		p := models.Performer{Name: "background update"}
		require.NoError(t, db.Performer.Create(sqlite.WithHistoryUser(ctx, testUserID),
			&models.CreatePerformerInput{Performer: &p}))

		// Strip the fixture user the harness attaches: this is how scan,
		// autotag, import and DLNA run. Such a task must not favourite on
		// somebody's behalf — an import carrying favorite:true from a JSON file
		// would otherwise write it to whoever happened to be first in the
		// users table.
		bgCtx := sqlite.WithHistoryUser(ctx, 0)

		p.Favorite = true
		require.NoError(t, db.Performer.Update(bgCtx, &models.UpdatePerformerInput{Performer: &p}))

		got, err := db.Performer.Find(sqlite.WithHistoryUser(ctx, testUserID), p.ID)
		require.NoError(t, err)
		assert.False(t, got.Favorite,
			"a background update must not create a favourite for any account")
	})
}

func TestStudioAndTagUpdateCarryFavourite(t *testing.T) {
	runWithRollbackTxn(t, "TestStudioAndTagUpdateCarryFavourite", func(t *testing.T, ctx context.Context) {
		ctx = sqlite.WithHistoryUser(ctx, testUserID)

		s := models.Studio{Name: "favourite studio"}
		require.NoError(t, db.Studio.Create(ctx, &s))
		s.Favorite = true
		require.NoError(t, db.Studio.Update(ctx, &s))

		gotStudio, err := db.Studio.Find(ctx, s.ID)
		require.NoError(t, err)
		assert.True(t, gotStudio.Favorite, "studio favourites must survive a full-object update")

		tg := models.Tag{Name: "favourite tag"}
		require.NoError(t, db.Tag.Create(ctx, &tg))
		tg.Favorite = true
		require.NoError(t, db.Tag.Update(ctx, &tg))

		gotTag, err := db.Tag.Find(ctx, tg.ID)
		require.NoError(t, err)
		assert.True(t, gotTag.Favorite, "tag favourites must survive a full-object update")
	})
}
