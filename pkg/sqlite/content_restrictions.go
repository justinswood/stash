package sqlite

import (
	"context"
	"strconv"
	"strings"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"

	"github.com/stashapp/stash/pkg/models"
)

type contentRestrictionsKey struct{}

// WithContentRestrictions returns a context carrying the content a request must
// not see. Attached per request from the caller's group membership. An absent
// or empty value means unrestricted, which is the case for every account that
// belongs to no group — and for every background task, which must keep seeing
// the whole library so that scans, generation and cleanup are not silently
// scoped to somebody's restrictions.
func WithContentRestrictions(ctx context.Context, r models.ContentRestrictions) context.Context {
	return context.WithValue(ctx, contentRestrictionsKey{}, r)
}

// contentRestrictions returns the restrictions for this request, if any.
func contentRestrictions(ctx context.Context) models.ContentRestrictions {
	if v, ok := ctx.Value(contentRestrictionsKey{}).(models.ContentRestrictions); ok {
		return v
	}
	return models.ContentRestrictions{}
}

// sqlIntList renders ints as a bare SQL list. Safe because the values are
// integers read from our own join tables, never user-supplied text — building
// them as bound parameters would require threading arg counts through every
// call site for no benefit.
func sqlIntList(ids []int) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.Itoa(id))
	}
	return strings.Join(parts, ",")
}

// sqlStringList renders strings as a quoted SQL list, with quotes doubled.
// Used for gender enum values, which are constrained by the enum but are still
// escaped rather than trusted.
func sqlStringList(vals []string) string {
	parts := make([]string, 0, len(vals))
	for _, v := range vals {
		parts = append(parts, "'"+strings.ReplaceAll(v, "'", "''")+"'")
	}
	return strings.Join(parts, ",")
}

// restrictPerformers hides performers of an excluded gender, or carrying an
// excluded tag.
//
// A NULL gender stays visible: unknown is not the same as excluded, and 2 of
// this library's performers have no gender recorded.
func restrictPerformers(ctx context.Context, q *queryBuilder) {
	r := contentRestrictions(ctx)
	if r.Empty() {
		return
	}

	if len(r.ExcludedGenders) > 0 {
		q.addWhere("(performers.gender IS NULL OR performers.gender NOT IN (" +
			sqlStringList(r.ExcludedGenders) + "))")
	}
	if len(r.ExcludedTagIDs) > 0 {
		q.addWhere("NOT EXISTS (SELECT 1 FROM performers_tags pt_r WHERE pt_r.performer_id = performers.id AND pt_r.tag_id IN (" +
			sqlIntList(r.ExcludedTagIDs) + "))")
	}
}

// restrictByPerformerAndTag hides rows in a content table that feature an
// excluded-gender performer or carry an excluded tag.
//
// idColumn is the table's primary key expression (e.g. "scenes.id"),
// performerJoin/tagJoin the join tables and their FK column.
func restrictByPerformerAndTag(
	ctx context.Context,
	q *queryBuilder,
	idColumn string,
	performerJoinTable, performerFK string,
	tagJoinTable, tagFK string,
) {
	r := contentRestrictions(ctx)
	if r.Empty() {
		return
	}

	if len(r.ExcludedGenders) > 0 {
		q.addWhere("NOT EXISTS (SELECT 1 FROM " + performerJoinTable + " pj_r" +
			" JOIN performers p_r ON p_r.id = pj_r.performer_id" +
			" WHERE pj_r." + performerFK + " = " + idColumn +
			" AND p_r.gender IN (" + sqlStringList(r.ExcludedGenders) + "))")
	}
	if len(r.ExcludedTagIDs) > 0 {
		q.addWhere("NOT EXISTS (SELECT 1 FROM " + tagJoinTable + " tj_r" +
			" WHERE tj_r." + tagFK + " = " + idColumn +
			" AND tj_r.tag_id IN (" + sqlIntList(r.ExcludedTagIDs) + "))")
	}
}

// restrictScenes hides scenes featuring an excluded-gender performer or
// carrying an excluded tag.
func restrictScenes(ctx context.Context, q *queryBuilder) {
	restrictByPerformerAndTag(ctx, q, "scenes.id",
		"performers_scenes", "scene_id", "scenes_tags", "scene_id")
}

func restrictImages(ctx context.Context, q *queryBuilder) {
	restrictByPerformerAndTag(ctx, q, "images.id",
		"performers_images", "image_id", "images_tags", "image_id")
}

func restrictGalleries(ctx context.Context, q *queryBuilder) {
	restrictByPerformerAndTag(ctx, q, "galleries.id",
		"performers_galleries", "gallery_id", "galleries_tags", "gallery_id")
}

// RestrictionsActive reports whether this request is scoped at all. Used by
// resolvers that need to re-check a single object fetched by id, where the
// query-builder filters do not apply.
func RestrictionsActive(ctx context.Context) bool {
	return !contentRestrictions(ctx).Empty()
}

// visibleByPerformerAndTag reports whether one row survives the current
// restrictions. The query-builder filters only cover list queries; fetching an
// object by id (findScene, findPerformer…) bypasses them entirely, so a
// restricted user could otherwise reach hidden content by navigating straight
// to its URL.
func visibleByPerformerAndTag(
	ctx context.Context,
	table string, id int,
	performerJoinTable, performerFK string,
	tagJoinTable, tagFK string,
) (bool, error) {
	r := contentRestrictions(ctx)
	if r.Empty() {
		return true, nil
	}

	idCol := table + ".id"
	conds := []exp.Expression{goqu.T(table).Col("id").Eq(id)}

	if len(r.ExcludedGenders) > 0 {
		conds = append(conds, goqu.L(
			"NOT EXISTS (SELECT 1 FROM "+performerJoinTable+" pj_v"+
				" JOIN performers p_v ON p_v.id = pj_v.performer_id"+
				" WHERE pj_v."+performerFK+" = "+idCol+
				" AND p_v.gender IN ("+sqlStringList(r.ExcludedGenders)+"))"))
	}
	if len(r.ExcludedTagIDs) > 0 {
		conds = append(conds, goqu.L(
			"NOT EXISTS (SELECT 1 FROM "+tagJoinTable+" tj_v"+
				" WHERE tj_v."+tagFK+" = "+idCol+
				" AND tj_v.tag_id IN ("+sqlIntList(r.ExcludedTagIDs)+"))"))
	}

	q := dialect.From(goqu.T(table)).Select(goqu.COUNT("*")).Where(conds...)
	var n int
	if err := querySimple(ctx, q, &n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// SceneVisible reports whether a scene may be seen by this request.
func SceneVisible(ctx context.Context, id int) (bool, error) {
	return visibleByPerformerAndTag(ctx, "scenes", id,
		"performers_scenes", "scene_id", "scenes_tags", "scene_id")
}

// ImageVisible reports whether an image may be seen by this request.
func ImageVisible(ctx context.Context, id int) (bool, error) {
	return visibleByPerformerAndTag(ctx, "images", id,
		"performers_images", "image_id", "images_tags", "image_id")
}

// GalleryVisible reports whether a gallery may be seen by this request.
func GalleryVisible(ctx context.Context, id int) (bool, error) {
	return visibleByPerformerAndTag(ctx, "galleries", id,
		"performers_galleries", "gallery_id", "galleries_tags", "gallery_id")
}

// PerformerVisible reports whether a performer may be seen by this request.
// A NULL gender stays visible — unknown is not the same as excluded.
func PerformerVisible(ctx context.Context, id int) (bool, error) {
	r := contentRestrictions(ctx)
	if r.Empty() {
		return true, nil
	}

	conds := []exp.Expression{goqu.T("performers").Col("id").Eq(id)}
	if len(r.ExcludedGenders) > 0 {
		conds = append(conds, goqu.L(
			"(performers.gender IS NULL OR performers.gender NOT IN ("+
				sqlStringList(r.ExcludedGenders)+"))"))
	}
	if len(r.ExcludedTagIDs) > 0 {
		conds = append(conds, goqu.L(
			"NOT EXISTS (SELECT 1 FROM performers_tags pt_v"+
				" WHERE pt_v.performer_id = performers.id"+
				" AND pt_v.tag_id IN ("+sqlIntList(r.ExcludedTagIDs)+"))"))
	}

	q := dialect.From(goqu.T("performers")).Select(goqu.COUNT("*")).Where(conds...)
	var n int
	if err := querySimple(ctx, q, &n); err != nil {
		return false, err
	}
	return n > 0, nil
}
