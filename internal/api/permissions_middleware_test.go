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

// TestRequiredRoleForQuery pins which reads are open to every authenticated
// role and which expose the host rather than the library. Before this existed,
// the middleware checked Mutation only and every query below was readable by
// any account, including READ_ONLY.
func TestRequiredRoleForQuery(t *testing.T) {
	cases := []struct {
		field string
		want  models.UserRole
		why   string
	}{
		{
			field: "findScenes",
			want:  models.UserRoleReadOnly,
			why:   "browsing the library is what every role is for; gating it would break READ_ONLY entirely",
		},
		{
			field: "stats",
			want:  models.UserRoleReadOnly,
			why:   "aggregate counts are part of browsing",
		},
		{
			field: "configuration",
			want:  models.UserRoleReadOnly,
			why: "the UI cannot render without it; it redacts its own secrets for " +
				"non-admins in resolver_query_configuration.go rather than being gated here",
		},
		{
			field: "directory",
			want:  models.UserRoleAdmin,
			why:   "walks arbitrary host filesystem paths — the most sensitive read in the schema",
		},
		{
			field: "findFiles",
			want:  models.UserRoleAdmin,
			why:   "returns real filesystem paths, which are not library content",
		},
		{
			field: "logs",
			want:  models.UserRoleAdmin,
			why:   "logs leak paths, config and request detail",
		},
		{
			field: "jobQueue",
			want:  models.UserRoleAdmin,
			why:   "exposes what the instance is doing and the paths it is doing it to",
		},
		{
			field: "systemStatus",
			want:  models.UserRoleAdmin,
			why: "reports database/config location. Safe to gate: the Setup and Migrate " +
				"flows that call it run with no credentials configured, or with the DB " +
				"not yet open, and both of those resolve to admin already",
		},
		{
			field: "listScrapers",
			want:  models.UserRoleAdmin,
			why:   "scraper config includes third-party endpoints",
		},
		{
			field: "scrapeURL",
			want:  models.UserRoleUser,
			why: "makes the server fetch an arbitrary URL on the caller's behalf; " +
				"READ_ONLY must not be able to drive outbound requests",
		},
		{
			field: "scrapeSingleScene",
			want:  models.UserRoleUser,
			why:   "scraping is part of editing metadata, which READ_ONLY cannot do",
		},
		{
			field: "findShareLinks",
			want:  models.UserRoleUser,
			why:   "matches the shareLink* mutations, which are USER-level",
		},
		{
			// Regression guard. PluginsLoader in App.tsx calls `plugins` on every
			// page load for every account to fetch the plugin JS/CSS list. Gating
			// it to admin breaks the app for every non-admin user — the loader
			// errors and plugin assets never load.
			field: "plugins",
			want:  models.UserRoleReadOnly,
			why:   "the app-wide plugin loader calls this for every account on every page load",
		},
		{
			field: "pluginTasks",
			want:  models.UserRoleAdmin,
			why:   "unlike `plugins`, this is only reached from Settings > Tasks",
		},
		{
			field: "someQueryAddedLater",
			want:  models.UserRoleReadOnly,
			why: "an unclassified query defaults to readable — deliberate, so adding a " +
				"content query cannot accidentally lock out READ_ONLY. A new query that " +
				"exposes the host must be added to adminQueries explicitly",
		},
	}

	for _, c := range cases {
		t.Run(c.field, func(t *testing.T) {
			if got := requiredRoleForQuery(c.field); got != c.want {
				t.Errorf("requiredRoleForQuery(%q) = %q, want %q\nwhy this matters: %s",
					c.field, got, c.want, c.why)
			}
		})
	}
}

// TestMeQueryIsExempt guards the one query that must answer for anyone at all.
// The UI calls `me` to discover whether a session exists; if it were gated,
// an unauthenticated caller would get a permission error instead of null and
// the client could not tell "logged out" from "broken".
func TestMeQueryIsExempt(t *testing.T) {
	if !exemptQueries["me"] {
		t.Error("`me` must bypass the role check so it can return null when logged out")
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
