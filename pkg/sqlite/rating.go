package sqlite

import (
	"context"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

// Ratings are per-user (migration 86), stored in join tables rather than the
// legacy `rating` column on each object.
//
// This mirrors favourites (migration 83) and per-user history (78), and uses
// the same acting user — WithHistoryUser, set by authenticateHandler for the
// GraphQL endpoint. With no user in context ratings read as unset and rating
// filters match nothing, which is deliberate: a scan or DLNA request must not
// inherit somebody's stars, and attributing one to "whoever the scanner is"
// would be worse than reporting none.
//
// Unlike favourites, a rating is a VALUE rather than a flag, so it also has to
// work in ORDER BY. Both the filter and the sort go through a correlated
// subquery (ratingValueSQL) rather than a join, so that objects the user has
// not rated still appear — a join would drop them from every rating sort.
const (
	sceneRatingsTable     = "scenes_ratings"
	imageRatingsTable     = "images_ratings"
	galleryRatingsTable   = "galleries_ratings"
	performerRatingsTable = "performers_ratings"
	studioRatingsTable    = "studios_ratings"
	groupRatingsTable     = "groups_ratings"
)

// ratingValueSQL renders a scalar subquery yielding the current user's rating
// for the row identified by idColumn, or NULL when they have not rated it.
//
// Returns literal NULL when there is no user in context, so a rating filter
// matches nothing and a rating sort is uniform rather than falling back to
// somebody else's stars.
//
// The user id is an int read from our own context, never user-supplied text,
// so inlining it needs no parameter binding — same reasoning as
// favoriteExistsSQL.
func ratingValueSQL(ctx context.Context, joinTable, fkColumn, idColumn string) string {
	uid := historyUserID(ctx)
	if uid <= 0 {
		return "NULL"
	}
	return fmt.Sprintf(
		"(SELECT rating_x.rating FROM %s rating_x WHERE rating_x.%s = %s AND rating_x.user_id = %d)",
		joinTable, fkColumn, idColumn, uid,
	)
}

// loadRatings returns the current user's rating for each of the given ids.
// One indexed query per result page rather than per row, matching
// loadFavorites.
func loadRatings(ctx context.Context, joinTable, fkColumn string, ids []int) (map[int]int, error) {
	ret := make(map[int]int, len(ids))
	uid := historyUserID(ctx)
	if uid <= 0 || len(ids) == 0 {
		return ret, nil
	}

	t := goqu.T(joinTable)
	q := dialect.From(t).Prepared(true).
		Select(t.Col(fkColumn), t.Col("rating")).
		Where(t.Col("user_id").Eq(uid), t.Col(fkColumn).In(ids))

	const single = false
	if err := queryFunc(ctx, q, single, func(r *sqlx.Rows) error {
		var id, rating int
		if err := r.Scan(&id, &rating); err != nil {
			return err
		}
		ret[id] = rating
		return nil
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

// applyRatings populates the per-user rating on a page of results. Called from
// each store's getMany, which is the single funnel every read path goes
// through — a read path that does not use it reports the dead legacy column.
func applyRatings[T any](ctx context.Context, joinTable, fkColumn string, items []T, id func(T) int, set func(T, *int)) error {
	if len(items) == 0 {
		return nil
	}

	ids := make([]int, 0, len(items))
	for _, o := range items {
		ids = append(ids, id(o))
	}

	ratings, err := loadRatings(ctx, joinTable, fkColumn, ids)
	if err != nil {
		return err
	}

	for _, o := range items {
		if r, ok := ratings[id(o)]; ok {
			v := r
			set(o, &v)
		} else {
			set(o, nil)
		}
	}
	return nil
}

// setRating records or clears the current user's rating. A nil rating removes
// the row, so "unrated" and "rated 0" stay distinguishable.
//
// A no-op when there is no user in context — a background task must not rate
// on somebody's behalf, the same rule setFavorite follows.
func setRating(ctx context.Context, joinTable, fkColumn string, id int, rating *int) error {
	uid := historyUserID(ctx)
	if uid <= 0 {
		return nil
	}

	t := goqu.T(joinTable)

	if rating == nil {
		q := dialect.Delete(t).Prepared(true).
			Where(t.Col("user_id").Eq(uid), t.Col(fkColumn).Eq(id))
		_, err := exec(ctx, q)
		return err
	}

	// The primary key is (user_id, <fk>), so an upsert keeps re-rating to one
	// statement instead of a read-then-write.
	q := dialect.Insert(t).Prepared(true).
		Rows(goqu.Record{"user_id": uid, fkColumn: id, "rating": *rating}).
		OnConflict(goqu.DoUpdate("user_id, "+fkColumn, goqu.Record{"rating": *rating}))

	_, err := exec(ctx, q)
	return err
}

// ratingSortSQL renders the ORDER BY fragment for a per-user rating sort.
// Mirrors the correlated-subquery form already used for last_played_at and
// last_o_at sorting.
func ratingSortSQL(ctx context.Context, joinTable, fkColumn, idColumn, direction string) string {
	return fmt.Sprintf(" ORDER BY %s %s",
		ratingValueSQL(ctx, joinTable, fkColumn, idColumn),
		getSortDirection(direction),
	)
}
