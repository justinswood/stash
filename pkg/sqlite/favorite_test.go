package sqlite

import (
	"context"
	"strings"
	"testing"
)

// TestFavoriteExistsSQLWithoutUser guards the case that decides whether
// favourites leak across accounts. Background tasks (scan, generate, autotag,
// DLNA) run with no user in context. If the generated predicate defaulted to
// "true" — or referenced a user id of 0 that happened to match a row — those
// tasks would see somebody else's favourites, and a filter run by the scanner
// would silently act on them.
func TestFavoriteExistsSQLWithoutUser(t *testing.T) {
	ctx := context.Background() // no history user

	got := favoriteExistsSQL(ctx, performerFavoritesTable, "performer_id", "performers.id")
	if got != "0" {
		t.Errorf("favoriteExistsSQL with no user = %q, want %q — a background task must "+
			"match no favourites rather than somebody else's", got, "0")
	}

	got = performerFavoriteExistsSQL(ctx, "performers_scenes", "scene_id", "scenes.id")
	if got != "0" {
		t.Errorf("performerFavoriteExistsSQL with no user = %q, want %q", got, "0")
	}
}

// TestFavoriteExistsSQLScopesToUser checks the predicate actually constrains on
// the user id from context, not just on the object id.
func TestFavoriteExistsSQLScopesToUser(t *testing.T) {
	ctx := WithHistoryUser(context.Background(), 7)

	got := favoriteExistsSQL(ctx, performerFavoritesTable, "performer_id", "performers.id")
	for _, want := range []string{
		"EXISTS",
		performerFavoritesTable,
		"user_id = 7",
		"performers.id",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("favoriteExistsSQL missing %q\ngot: %s", want, got)
		}
	}
}

// TestPerformerFavoriteExistsSQLJoinsThroughLinkTable checks the scene/gallery/
// image variant reaches performers via the link table and constrains the user.
// This replaced a join-and-SUM construction; getting the user constraint into
// the wrong half of the join would make it match every account's favourites.
func TestPerformerFavoriteExistsSQLJoinsThroughLinkTable(t *testing.T) {
	ctx := WithHistoryUser(context.Background(), 3)

	got := performerFavoriteExistsSQL(ctx, "performers_scenes", "scene_id", "scenes.id")
	for _, want := range []string{
		"performers_scenes",
		performerFavoritesTable,
		"user_id = 3",
		"scenes.id",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("performerFavoriteExistsSQL missing %q\ngot: %s", want, got)
		}
	}
}
