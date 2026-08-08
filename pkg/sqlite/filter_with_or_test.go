//go:build integration
// +build integration

package sqlite_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stashapp/stash/pkg/models"
)

// A performer_tags criterion registers a CTE (`WITH performer_tags AS (...)`)
// and a join against it. Joins were collected across sub-filters but WITH
// clauses were not, so nesting the criterion inside an OR emitted a join
// against a table that had never been defined and the query failed with
// "no such table: performer_tags".
//
// The shape matters: the criterion alone worked, and a plain OR of ordinary
// criteria worked, so this only ever appeared in the combination — which is why
// it survived as "unfixed upstream issue" rather than being caught by the
// existing filter tests. It blocked the obvious single-query union of "scene
// tagged X OR scene whose performer is tagged X".
//
// Affects images and galleries identically, since they share the handler.

func performerTagsCriterion(tagID int) *models.HierarchicalMultiCriterionInput {
	return &models.HierarchicalMultiCriterionInput{
		Value:    []string{intPtrToString(tagID)},
		Modifier: models.CriterionModifierIncludes,
	}
}

func intPtrToString(i int) string {
	return strconv.Itoa(i)
}

func TestSceneQueryPerformerTagsInsideOr(t *testing.T) {
	runWithRollbackTxn(t, "TestSceneQueryPerformerTagsInsideOr", func(t *testing.T, ctx context.Context) {
		tagID := tagIDs[tagIdxWithPerformer]

		// performer_tags nested inside an OR — the combination that crashed.
		filter := models.SceneFilterType{
			Title: &models.StringCriterionInput{
				Value:    "not-a-real-title-xyz",
				Modifier: models.CriterionModifierEquals,
			},
			OperatorFilter: models.OperatorFilter[models.SceneFilterType]{
				Or: &models.SceneFilterType{
					PerformerTags: performerTagsCriterion(tagID),
				},
			},
		}

		res, err := db.Scene.Query(ctx, models.SceneQueryOptions{
			SceneFilter:  &filter,
			QueryOptions: models.QueryOptions{Count: true},
		})
		require.NoError(t, err,
			"performer_tags inside an OR must not fail — the CTE has to be collected "+
				"from the sub-filter, not just the top-level filter")

		// The OR side should match the scenes whose performers carry the tag,
		// since the title side matches nothing.
		assert.Greater(t, res.Count, 0,
			"the OR branch should still select scenes via performer tags")
	})
}

// The same criterion on both sides generates a byte-identical CTE. Without
// de-duplication that emits `performer_tags AS (...)` twice and SQLite rejects
// the duplicate name — a different failure from the one above, reachable once
// sub-filter clauses are collected.
func TestSceneQueryPerformerTagsBothSidesOfOr(t *testing.T) {
	runWithRollbackTxn(t, "TestSceneQueryPerformerTagsBothSidesOfOr", func(t *testing.T, ctx context.Context) {
		tagID := tagIDs[tagIdxWithPerformer]

		filter := models.SceneFilterType{
			PerformerTags: performerTagsCriterion(tagID),
			OperatorFilter: models.OperatorFilter[models.SceneFilterType]{
				Or: &models.SceneFilterType{
					PerformerTags: performerTagsCriterion(tagID),
				},
			},
		}

		_, err := db.Scene.Query(ctx, models.SceneQueryOptions{
			SceneFilter:  &filter,
			QueryOptions: models.QueryOptions{Count: true},
		})
		assert.NoError(t, err,
			"an identical CTE on both sides must be collapsed, not emitted twice")
	})
}

func TestImageQueryPerformerTagsInsideOr(t *testing.T) {
	runWithRollbackTxn(t, "TestImageQueryPerformerTagsInsideOr", func(t *testing.T, ctx context.Context) {
		tagID := tagIDs[tagIdxWithPerformer]

		filter := models.ImageFilterType{
			Title: &models.StringCriterionInput{
				Value:    "not-a-real-title-xyz",
				Modifier: models.CriterionModifierEquals,
			},
			OperatorFilter: models.OperatorFilter[models.ImageFilterType]{
				Or: &models.ImageFilterType{
					PerformerTags: performerTagsCriterion(tagID),
				},
			},
		}

		_, err := db.Image.Query(ctx, models.ImageQueryOptions{
			ImageFilter:  &filter,
			QueryOptions: models.QueryOptions{Count: true},
		})
		assert.NoError(t, err, "images share the handler and must not fail either")
	})
}

func TestGalleryQueryPerformerTagsInsideOr(t *testing.T) {
	runWithRollbackTxn(t, "TestGalleryQueryPerformerTagsInsideOr", func(t *testing.T, ctx context.Context) {
		tagID := tagIDs[tagIdxWithPerformer]

		filter := models.GalleryFilterType{
			Title: &models.StringCriterionInput{
				Value:    "not-a-real-title-xyz",
				Modifier: models.CriterionModifierEquals,
			},
			OperatorFilter: models.OperatorFilter[models.GalleryFilterType]{
				Or: &models.GalleryFilterType{
					PerformerTags: performerTagsCriterion(tagID),
				},
			},
		}

		_, _, err := db.Gallery.Query(ctx, &filter, nil)
		assert.NoError(t, err, "galleries share the handler and must not fail either")
	})
}
