package sqlite

import (
	"context"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

// Favourites are per-user (migration 83). They are stored in join tables rather
// than the legacy `favorite` column on each object, which is no longer read or
// written.
//
// The acting user comes from the same context value as per-user history
// (WithHistoryUser), which authenticateHandler sets for the GraphQL endpoint.
// When it is absent — background tasks, the DLNA server, anything not acting on
// behalf of a signed-in person — favourites resolve to false and filters that
// depend on them match nothing. That is deliberate: attributing a favourite to
// "whoever the scanner is" would be worse than reporting none.

const (
	performerFavoritesTable = "performers_favorites"
	studioFavoritesTable    = "studios_favorites"
	tagFavoritesTable       = "tags_favorites"
)

// favoriteExistsSQL renders a boolean expression that is true when the current
// user has favourited the row identified by idColumn. Returns "0" (never true)
// when no user is in context.
//
// The user id is an int read from our own context, never user-supplied text, so
// inlining it needs no parameter binding.
func favoriteExistsSQL(ctx context.Context, joinTable, fkColumn, idColumn string) string {
	uid := historyUserID(ctx)
	if uid <= 0 {
		return "0"
	}
	return fmt.Sprintf(
		"EXISTS (SELECT 1 FROM %s fav_x WHERE fav_x.%s = %s AND fav_x.user_id = %d)",
		joinTable, fkColumn, idColumn, uid,
	)
}

// loadFavorites returns which of the given ids the current user has favourited.
// One indexed query per result page rather than per row.
func loadFavorites(ctx context.Context, joinTable, fkColumn string, ids []int) (map[int]bool, error) {
	ret := make(map[int]bool, len(ids))
	uid := historyUserID(ctx)
	if uid <= 0 || len(ids) == 0 {
		return ret, nil
	}

	t := goqu.T(joinTable)
	q := dialect.From(t).Prepared(true).
		Select(t.Col(fkColumn)).
		Where(t.Col("user_id").Eq(uid), t.Col(fkColumn).In(ids))

	const single = false
	if err := queryFunc(ctx, q, single, func(r *sqlx.Rows) error {
		var id int
		if err := r.Scan(&id); err != nil {
			return err
		}
		ret[id] = true
		return nil
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

// setFavorite adds or removes a favourite for the current user. A no-op when
// there is no user in context — a background task must not silently favourite
// something on somebody's behalf.
func setFavorite(ctx context.Context, joinTable, fkColumn string, id int, favorite bool) error {
	uid := historyUserID(ctx)
	if uid <= 0 {
		return nil
	}

	t := goqu.T(joinTable)
	if !favorite {
		q := dialect.Delete(t).Prepared(true).
			Where(t.Col("user_id").Eq(uid), t.Col(fkColumn).Eq(id))
		_, err := exec(ctx, q)
		return err
	}

	// INSERT OR IGNORE: favouriting twice is not an error, and the primary key
	// is what makes it idempotent.
	q := dialect.Insert(t).Prepared(true).
		Rows(goqu.Record{"user_id": uid, fkColumn: id}).
		OnConflict(goqu.DoNothing())
	_, err := exec(ctx, q)
	return err
}

// performerFavoriteExistsSQL renders a boolean expression that is true when the
// row identified by idColumn has at least one performer the current user has
// favourited.
//
// Replaces the previous join-and-SUM construction: NOT(EXISTS) expresses
// "contains zero favourites" directly, and gets the no-performers case right
// without a separate IS NULL branch.
func performerFavoriteExistsSQL(ctx context.Context, linkTable, fkColumn, idColumn string) string {
	uid := historyUserID(ctx)
	if uid <= 0 {
		return "0"
	}
	return fmt.Sprintf(
		"EXISTS (SELECT 1 FROM %s pfx_x JOIN %s favp_x ON favp_x.performer_id = pfx_x.performer_id "+
			"AND favp_x.user_id = %d WHERE pfx_x.%s = %s)",
		linkTable, performerFavoritesTable, uid, fkColumn, idColumn,
	)
}
