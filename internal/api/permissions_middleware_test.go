package api

import (
	"testing"

	"github.com/stashapp/stash/pkg/models"
)

// TestRequiredRoleForMutation pins the permission policy itself, not the
// mechanics of the lookup. Each case below is a decision about who may do what;
// if one of them changes, that is a security-relevant change and should have to
// be made deliberately.
func TestRequiredRoleForMutation(t *testing.T) {
	cases := []struct {
		field string
		want  models.UserRole
		why   string
	}{
		{
			field: "sceneAddO",
			want:  models.UserRoleReadOnly,
			why:   "recording your own history is the whole point of the READ_ONLY role",
		},
		{
			field: "changePassword",
			want:  models.UserRoleReadOnly,
			why:   "every account must be able to rotate its own password",
		},
		{
			field: "configureGeneral",
			want:  models.UserRoleAdmin,
			why:   "instance configuration is admin-only",
		},
		{
			field: "userCreate",
			want:  models.UserRoleAdmin,
			why:   "user management is admin-only; otherwise a USER could mint an admin",
		},
		{
			field: "sceneDestroy",
			want:  models.UserRoleAdmin,
			why:   "the USER role does not delete content; the *Destroy suffix enforces this",
		},
		{
			field: "sceneUpdate",
			want:  models.UserRoleUser,
			why:   "metadata editing is the USER role's core capability",
		},
		{
			field: "someMutationAddedLater",
			want:  models.UserRoleUser,
			why:   "an unclassified mutation must never default to admin, and must never be open",
		},
		{
			field: "userAPIKeyCreate",
			want:  models.UserRoleUser,
			why:   "a user issues their own API keys; the resolver requires admin only to target another account",
		},
		{
			// Regression guard. Renaming this to userAPIKeyDestroy would make it
			// admin-only via the *Destroy suffix rule and silently stop users
			// revoking their own keys — a lockout that looks like a UI bug.
			field: "userAPIKeyRevoke",
			want:  models.UserRoleUser,
			why:   "users must be able to revoke their own keys without an admin",
		},
	}

	for _, c := range cases {
		t.Run(c.field, func(t *testing.T) {
			if got := requiredRoleForMutation(c.field); got != c.want {
				t.Errorf("requiredRoleForMutation(%q) = %q, want %q\nwhy this matters: %s",
					c.field, got, c.want, c.why)
			}
		})
	}
}

// TestDestroySuffixDoesNotOverrideExemptions guards the ordering inside
// requiredRoleForMutation: the userAPIKey and shareLink prefix rules must be
// evaluated before the *Destroy suffix rule, or those features become admin-only.
func TestDestroySuffixDoesNotOverrideExemptions(t *testing.T) {
	if got := requiredRoleForMutation("shareLinkDestroy"); got != models.UserRoleUser {
		t.Errorf("shareLinkDestroy = %q, want USER: share links are a user-level feature, "+
			"so the shareLink prefix rule must win over the *Destroy suffix rule", got)
	}
}
