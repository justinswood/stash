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

// Saved filters became per-user in migration 84. Before it, saved_filters had
// no owner column at all, so every account saw — and could edit or delete —
// every other account's filters. On this instance that surfaced as a
// second account opening the home page and being shown the admin's "Favs"
// filter row.
//
// Reads funnel through selectDataset and writes go through tableMgr, which
// never sees that predicate. Both halves are covered here: a leak on the read
// side shows another account's filters, a leak on the write side lets an
// account overwrite or delete a filter it cannot even see.

func makeUser(ctx context.Context, t *testing.T, username string) int {
	t.Helper()
	u := models.User{
		Username:     username,
		PasswordHash: "not-a-real-hash",
		Role:         models.UserRoleUser,
	}
	require.NoError(t, db.User.Create(ctx, &u), "creating user %q", username)
	return u.ID
}

func makeFilter(ctx context.Context, t *testing.T, name string) *models.SavedFilter {
	t.Helper()
	f := models.SavedFilter{
		Mode: models.FilterModeScenes,
		Name: name,
	}
	require.NoError(t, db.SavedFilter.Create(ctx, &f), "creating filter %q", name)
	return &f
}

func TestSavedFilterNotVisibleToOtherUser(t *testing.T) {
	runWithRollbackTxn(t, "TestSavedFilterNotVisibleToOtherUser", func(t *testing.T, ctx context.Context) {
		aID := makeUser(ctx, t, "filter-owner-a")
		bID := makeUser(ctx, t, "filter-owner-b")

		aCtx := sqlite.WithHistoryUser(ctx, aID)
		bCtx := sqlite.WithHistoryUser(ctx, bID)

		f := makeFilter(aCtx, t, "A's filter")

		// Find: the direct fetch the front page uses to resolve a saved-filter
		// row by id. Returning it here is what put the admin's "Favs" row on
		// another account's home page.
		got, err := db.SavedFilter.Find(bCtx, f.ID)
		require.NoError(t, err)
		assert.Nil(t, got, "one account's saved filter must not be findable by another")

		got, err = db.SavedFilter.Find(aCtx, f.ID)
		require.NoError(t, err)
		require.NotNil(t, got, "the owner must still see their own filter")
		assert.Equal(t, "A's filter", got.Name)

		// FindByMode: what populates the filter dropdown on a list page.
		list, err := db.SavedFilter.FindByMode(bCtx, models.FilterModeScenes)
		require.NoError(t, err)
		for _, l := range list {
			assert.NotEqual(t, f.ID, l.ID, "another account's filter must not appear in the dropdown")
		}

		list, err = db.SavedFilter.FindByMode(aCtx, models.FilterModeScenes)
		require.NoError(t, err)
		found := false
		for _, l := range list {
			if l.ID == f.ID {
				found = true
			}
		}
		assert.True(t, found, "the owner's filter must appear in their own dropdown")

		// All: used by the settings/export paths.
		all, err := db.SavedFilter.All(bCtx)
		require.NoError(t, err)
		for _, l := range all {
			assert.NotEqual(t, f.ID, l.ID, "another account's filter must not appear in All")
		}
	})
}

func TestSavedFilterCannotBeUpdatedByOtherUser(t *testing.T) {
	runWithRollbackTxn(t, "TestSavedFilterCannotBeUpdatedByOtherUser", func(t *testing.T, ctx context.Context) {
		aID := makeUser(ctx, t, "update-owner-a")
		bID := makeUser(ctx, t, "update-owner-b")

		aCtx := sqlite.WithHistoryUser(ctx, aID)
		bCtx := sqlite.WithHistoryUser(ctx, bID)

		f := makeFilter(aCtx, t, "A's filter")

		// Writes bypass selectDataset entirely — updateByID works on the raw id.
		// Scoping reads alone would leave this hole open.
		hijack := models.SavedFilter{ID: f.ID, Mode: models.FilterModeScenes, Name: "B's rewrite"}
		err := db.SavedFilter.Update(bCtx, &hijack)
		assert.Error(t, err, "an account must not be able to rewrite a filter it cannot see")

		still, err := db.SavedFilter.Find(aCtx, f.ID)
		require.NoError(t, err)
		require.NotNil(t, still)
		assert.Equal(t, "A's filter", still.Name, "the owner's filter must be unchanged")
	})
}

func TestSavedFilterCannotBeDestroyedByOtherUser(t *testing.T) {
	runWithRollbackTxn(t, "TestSavedFilterCannotBeDestroyedByOtherUser", func(t *testing.T, ctx context.Context) {
		aID := makeUser(ctx, t, "destroy-owner-a")
		bID := makeUser(ctx, t, "destroy-owner-b")

		aCtx := sqlite.WithHistoryUser(ctx, aID)
		bCtx := sqlite.WithHistoryUser(ctx, bID)

		f := makeFilter(aCtx, t, "A's filter")

		err := db.SavedFilter.Destroy(bCtx, f.ID)
		assert.Error(t, err, "an account must not be able to delete a filter it cannot see")

		still, err := db.SavedFilter.Find(aCtx, f.ID)
		require.NoError(t, err)
		assert.NotNil(t, still, "the owner's filter must survive another account's delete")
	})
}

// Two accounts must be able to use the same filter name. The pre-migration
// unique index was on (mode, name) alone, which would reject this — and
// "Favs" is exactly the name people collide on.
func TestSavedFilterSameNameForTwoUsers(t *testing.T) {
	runWithRollbackTxn(t, "TestSavedFilterSameNameForTwoUsers", func(t *testing.T, ctx context.Context) {
		aID := makeUser(ctx, t, "samename-a")
		bID := makeUser(ctx, t, "samename-b")

		aCtx := sqlite.WithHistoryUser(ctx, aID)
		bCtx := sqlite.WithHistoryUser(ctx, bID)

		fa := makeFilter(aCtx, t, "Favs")

		fb := models.SavedFilter{Mode: models.FilterModeScenes, Name: "Favs"}
		require.NoError(t, db.SavedFilter.Create(bCtx, &fb),
			"two accounts must each be able to have a filter named Favs")

		assert.NotEqual(t, fa.ID, fb.ID, "they must be distinct rows")

		gotA, err := db.SavedFilter.Find(aCtx, fa.ID)
		require.NoError(t, err)
		require.NotNil(t, gotA)

		gotB, err := db.SavedFilter.Find(bCtx, fb.ID)
		require.NoError(t, err)
		require.NotNil(t, gotB)
	})
}

// An instance with authentication disabled has no accounts at all, and
// authenticateHandler lets requests through with no user. Those filters are
// stored unowned and must keep working — otherwise this migration would make
// every saved filter vanish on a single-user install.
func TestSavedFilterUnownedWhenNoUser(t *testing.T) {
	runWithRollbackTxn(t, "TestSavedFilterUnownedWhenNoUser", func(t *testing.T, ctx context.Context) {
		// ctx carries no history user
		f := makeFilter(ctx, t, "unowned filter")

		got, err := db.SavedFilter.Find(ctx, f.ID)
		require.NoError(t, err)
		require.NotNil(t, got, "an unowned filter must be readable with no user in context")
		assert.Equal(t, "unowned filter", got.Name)

		// It must not bleed into a real account's view.
		userID := makeUser(ctx, t, "someone")
		userCtx := sqlite.WithHistoryUser(ctx, userID)

		got, err = db.SavedFilter.Find(userCtx, f.ID)
		require.NoError(t, err)
		assert.Nil(t, got, "an unowned filter must not appear for an authenticated account")
	})
}
