//go:build integration
// +build integration

package sqlite_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sqlite"
)

// PerformerVisible is the guard behind both the findPerformer resolver and the
// /performer/{id}/image route. These tests pin the decisions that determine
// whether a restricted account can reach a performer it must not see — an
// answer of "visible" that should be "hidden" is a content leak, and the
// reverse hides a performer from an account entitled to see them.

// createPerformerWithGender makes a performer to assert against. Gender is a
// pointer so that the nil case — a performer with no gender recorded — can be
// expressed, which is the case the NULL rule below turns on.
func createPerformerWithGender(ctx context.Context, t *testing.T, name string, gender *models.GenderEnum) int {
	t.Helper()

	p := models.Performer{
		Name:   name,
		Gender: gender,
	}
	if err := db.Performer.Create(ctx, &models.CreatePerformerInput{Performer: &p}); err != nil {
		t.Fatalf("creating performer %q: %v", name, err)
	}
	return p.ID
}

func TestPerformerVisibleUnrestricted(t *testing.T) {
	runWithRollbackTxn(t, "TestPerformerVisibleUnrestricted", func(t *testing.T, ctx context.Context) {
		gender := models.GenderEnumTransgenderFemale
		id := createPerformerWithGender(ctx, t, "TestPerformerVisibleUnrestricted", &gender)

		// No restrictions attached. This is the path every admin request and —
		// more importantly — every background task takes. A scan or autotag run
		// scoped to somebody's restrictions would corrupt the library rather
		// than merely hide from it, so "no restrictions" must mean "see all".
		visible, err := sqlite.PerformerVisible(ctx, id)
		assert.NoError(t, err)
		assert.True(t, visible, "a performer must be visible when no restrictions are attached, "+
			"otherwise background tasks would be scoped to a user's restrictions")
	})
}

func TestPerformerVisibleExcludedGender(t *testing.T) {
	runWithRollbackTxn(t, "TestPerformerVisibleExcludedGender", func(t *testing.T, ctx context.Context) {
		excluded := models.GenderEnumTransgenderFemale
		included := models.GenderEnumFemale

		excludedID := createPerformerWithGender(ctx, t, "TestPerformerVisibleExcludedGender excluded", &excluded)
		includedID := createPerformerWithGender(ctx, t, "TestPerformerVisibleExcludedGender included", &included)
		nullID := createPerformerWithGender(ctx, t, "TestPerformerVisibleExcludedGender null", nil)

		rctx := sqlite.WithContentRestrictions(ctx, models.ContentRestrictions{
			ExcludedGenders: []string{excluded.String()},
		})

		visible, err := sqlite.PerformerVisible(rctx, excludedID)
		assert.NoError(t, err)
		assert.False(t, visible, "a performer of an excluded gender must be hidden")

		visible, err = sqlite.PerformerVisible(rctx, includedID)
		assert.NoError(t, err)
		assert.True(t, visible, "a performer of a non-excluded gender must stay visible")

		// Unknown is not the same as excluded. A restriction names the genders a
		// group must not see; a performer whose gender was never recorded has
		// not been shown to be one of them, so excluding them would hide
		// library content on no evidence.
		visible, err = sqlite.PerformerVisible(rctx, nullID)
		assert.NoError(t, err)
		assert.True(t, visible, "a performer with no gender recorded must stay visible — "+
			"unknown is not the same as excluded")
	})
}

func TestPerformerVisibleExcludedTag(t *testing.T) {
	runWithRollbackTxn(t, "TestPerformerVisibleExcludedTag", func(t *testing.T, ctx context.Context) {
		// performerIdxWithTag carries tagIdxWithPerformer; excluding that tag
		// must hide them even though their gender is not excluded.
		performerID := performerIDs[performerIdxWithTag]
		tagID := tagIDs[tagIdxWithPerformer]

		rctx := sqlite.WithContentRestrictions(ctx, models.ContentRestrictions{
			ExcludedTagIDs: []int{tagID},
		})

		visible, err := sqlite.PerformerVisible(rctx, performerID)
		assert.NoError(t, err)
		assert.False(t, visible, "a performer carrying an excluded tag must be hidden")

		// A performer with no tags at all must not be caught by the tag rule.
		untaggedID := performerIDs[performerIdxWithScene]
		visible, err = sqlite.PerformerVisible(rctx, untaggedID)
		assert.NoError(t, err)
		assert.True(t, visible, "a performer not carrying the excluded tag must stay visible")
	})
}

// RestrictionsActive is what every media route branches on before paying for a
// visibility query. If it reported false for a genuinely restricted request the
// route would skip its guard entirely, which is the exact shape of the
// /performer/{id}/image leak.
func TestRestrictionsActive(t *testing.T) {
	assert.False(t, sqlite.RestrictionsActive(context.Background()),
		"a request with no restrictions attached must not be treated as restricted")

	empty := sqlite.WithContentRestrictions(context.Background(), models.ContentRestrictions{})
	assert.False(t, sqlite.RestrictionsActive(empty),
		"an empty restriction set must not be treated as restricted")

	active := sqlite.WithContentRestrictions(context.Background(), models.ContentRestrictions{
		ExcludedGenders: []string{models.GenderEnumTransgenderFemale.String()},
	})
	assert.True(t, sqlite.RestrictionsActive(active),
		"a non-empty restriction set must be treated as restricted, or media routes skip their guard")
}
