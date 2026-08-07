//go:build integration
// +build integration

package sqlite_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Interface configuration became per-account in migration 85. Before it, the
// front page layout, default filters and theme settings lived in one
// instance-wide blob, so every account was served the admin's home page —
// which on this instance meant a second user opening the site and being shown
// the admin's saved-filter row.

func TestUserUIConfigRoundTrip(t *testing.T) {
	runWithRollbackTxn(t, "TestUserUIConfigRoundTrip", func(t *testing.T, ctx context.Context) {
		userID := makeUser(ctx, t, "uiconfig-roundtrip")

		cfg := map[string]interface{}{
			"frontPageContent": []interface{}{
				map[string]interface{}{"__typename": "SavedFilter", "savedFilterId": "1"},
			},
			"advancedMode": true,
		}
		require.NoError(t, db.UserUIConfig.Set(ctx, userID, cfg))

		got, err := db.UserUIConfig.Get(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, true, got["advancedMode"])
		assert.NotNil(t, got["frontPageContent"], "nested structures must survive the JSON round trip")
	})
}

// nil and empty are different answers and the resolver treats them
// differently: nil means "never customised" (serve stock defaults), empty means
// "deliberately cleared". Collapsing them would make a user who cleared their
// front page get defaults back on every reload.
func TestUserUIConfigAbsentIsNilNotEmpty(t *testing.T) {
	runWithRollbackTxn(t, "TestUserUIConfigAbsentIsNilNotEmpty", func(t *testing.T, ctx context.Context) {
		userID := makeUser(ctx, t, "uiconfig-absent")

		got, err := db.UserUIConfig.Get(ctx, userID)
		require.NoError(t, err)
		assert.Nil(t, got, "an account that has never saved settings must read as nil")

		require.NoError(t, db.UserUIConfig.Set(ctx, userID, map[string]interface{}{}))

		got, err = db.UserUIConfig.Get(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, got, "an account that cleared its settings must read as empty, not nil")
		assert.Len(t, got, 0)
	})
}

func TestUserUIConfigIsolatedBetweenUsers(t *testing.T) {
	runWithRollbackTxn(t, "TestUserUIConfigIsolatedBetweenUsers", func(t *testing.T, ctx context.Context) {
		aID := makeUser(ctx, t, "uiconfig-a")
		bID := makeUser(ctx, t, "uiconfig-b")

		require.NoError(t, db.UserUIConfig.Set(ctx, aID, map[string]interface{}{
			"frontPageContent": "A's layout",
		}))

		// This is the reported symptom: B opening the site and seeing A's home
		// page. B has saved nothing, so B must have nothing.
		got, err := db.UserUIConfig.Get(ctx, bID)
		require.NoError(t, err)
		assert.Nil(t, got, "one account's front page must not appear for another")

		require.NoError(t, db.UserUIConfig.Set(ctx, bID, map[string]interface{}{
			"frontPageContent": "B's layout",
		}))

		gotA, err := db.UserUIConfig.Get(ctx, aID)
		require.NoError(t, err)
		assert.Equal(t, "A's layout", gotA["frontPageContent"], "B's save must not overwrite A")

		gotB, err := db.UserUIConfig.Get(ctx, bID)
		require.NoError(t, err)
		assert.Equal(t, "B's layout", gotB["frontPageContent"])
	})
}

// Set is an upsert on the user_id primary key. Without that, saving twice —
// which the UI does on every settings change — would fail on the second write.
func TestUserUIConfigSetIsIdempotent(t *testing.T) {
	runWithRollbackTxn(t, "TestUserUIConfigSetIsIdempotent", func(t *testing.T, ctx context.Context) {
		userID := makeUser(ctx, t, "uiconfig-upsert")

		require.NoError(t, db.UserUIConfig.Set(ctx, userID, map[string]interface{}{"title": "first"}))
		require.NoError(t, db.UserUIConfig.Set(ctx, userID, map[string]interface{}{"title": "second"}))

		got, err := db.UserUIConfig.Get(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, "second", got["title"], "a second save must replace the first, not fail")
	})
}

// With no account — an instance running without authentication — there is no
// row to read or write, and the resolver falls back to the instance config
// file. The store must be a quiet no-op rather than an error, or such an
// install would fail on every settings save.
func TestUserUIConfigNoUserIsNoOp(t *testing.T) {
	runWithRollbackTxn(t, "TestUserUIConfigNoUserIsNoOp", func(t *testing.T, ctx context.Context) {
		require.NoError(t, db.UserUIConfig.Set(ctx, 0, map[string]interface{}{"title": "ignored"}))

		got, err := db.UserUIConfig.Get(ctx, 0)
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

// Deleting an account must take its interface configuration with it, rather
// than leaving a row that a recycled user id would inherit.
func TestUserUIConfigCascadesOnUserDelete(t *testing.T) {
	runWithRollbackTxn(t, "TestUserUIConfigCascadesOnUserDelete", func(t *testing.T, ctx context.Context) {
		userID := makeUser(ctx, t, "uiconfig-cascade")
		require.NoError(t, db.UserUIConfig.Set(ctx, userID, map[string]interface{}{"title": "doomed"}))

		require.NoError(t, db.User.Destroy(ctx, userID))

		got, err := db.UserUIConfig.Get(ctx, userID)
		require.NoError(t, err)
		assert.Nil(t, got, "the deleted account's configuration must be gone")
	})
}
