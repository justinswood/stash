package api

import (
	"testing"

	"github.com/stashapp/stash/pkg/models"
)

// TestRequiredCapabilityForMutation pins the permission policy itself. Each case
// is a decision about who may do what; if one changes, that is a
// security-relevant change and should have to be made deliberately.
func TestRequiredCapabilityForMutation(t *testing.T) {
	cases := []struct {
		field string
		want  models.Capability
		why   string
	}{
		{
			field: "sceneAddO",
			want:  models.CapOwnHistory,
			why:   "recording your own history is the whole point of the READ_ONLY role",
		},
		{
			field: "changePassword",
			want:  models.CapChangeOwnPassword,
			why:   "every account must be able to rotate its own password",
		},
		{
			field: "sceneUpdate",
			want:  models.CapEditMetadata,
			why:   "metadata editing is the USER role's core capability",
		},
		{
			field: "sceneDestroy",
			want:  models.CapDeleteContent,
			why:   "the *Destroy suffix routes destructive operations away from plain editing",
		},
		{
			field: "deleteFiles",
			want:  models.CapDeleteContent,
			why: "does not end in Destroy, so the suffix rule cannot catch it — it must " +
				"stay explicitly listed or it would fall through to EDIT_METADATA",
		},
		{
			field: "moveFiles",
			want:  models.CapDeleteContent,
			why:   "same trap as deleteFiles",
		},
		{
			field: "configureGeneral",
			want:  models.CapConfigure,
			why:   "instance configuration is separable from running tasks",
		},
		{
			field: "metadataScan",
			want:  models.CapRunTasks,
			why:   "someone may be trusted to scan the library without being trusted to reconfigure it",
		},
		{
			field: "execSQL",
			want:  models.CapExecuteSQL,
			why:   "arbitrary SQL is worth revoking even from most admins",
		},
		{
			field: "userCreate",
			want:  models.CapManageUsers,
			why:   "otherwise any account could mint an admin",
		},
		{
			field: "installPackages",
			want:  models.CapManagePlugins,
			why:   "installing code on the host is separable from other admin duties",
		},
		{
			field: "someMutationAddedLater",
			want:  models.CapEditMetadata,
			why: "an unclassified mutation must default to the least privileged thing " +
				"that still makes sense — never to an admin capability, and never open",
		},
		{
			field: "shareLinkCreate",
			want:  models.CapManageShares,
			why:   "share links publish content to the open internet with no authentication",
		},
		{
			// Regression guard. shareLinkDestroy must resolve via the shareLink
			// prefix, not the *Destroy suffix, or revoking a share would require
			// DELETE_CONTENT.
			field: "shareLinkDestroy",
			want:  models.CapManageShares,
			why:   "the shareLink prefix rule must be evaluated before the *Destroy suffix rule",
		},
		{
			field: "userAPIKeyCreate",
			want:  models.CapManageOwnAPIKeys,
			why:   "a user issues their own keys; the resolver requires MANAGE_USERS to target another account",
		},
		{
			// Regression guard. Renaming this to userAPIKeyDestroy would route it
			// to CapDeleteContent via the suffix rule and stop users revoking
			// their own keys.
			field: "userAPIKeyRevoke",
			want:  models.CapManageOwnAPIKeys,
			why:   "users must be able to revoke their own keys without admin rights",
		},
	}

	for _, c := range cases {
		t.Run(c.field, func(t *testing.T) {
			if got := requiredCapabilityForMutation(c.field); got != c.want {
				t.Errorf("requiredCapabilityForMutation(%q) = %q, want %q\nwhy this matters: %s",
					c.field, got, c.want, c.why)
			}
		})
	}
}

// TestRequiredCapabilityForQuery pins which reads are open to every account and
// which expose the host rather than the library.
func TestRequiredCapabilityForQuery(t *testing.T) {
	cases := []struct {
		field string
		want  models.Capability
		why   string
	}{
		{
			field: "findScenes",
			want:  models.CapViewLibrary,
			why:   "browsing the library is what every role is for",
		},
		{
			field: "configuration",
			want:  models.CapViewLibrary,
			why: "the UI cannot render without it; it redacts its own secrets for " +
				"non-admins in resolver_query_configuration.go rather than being gated here",
		},
		{
			// Regression guard. PluginsLoader in App.tsx calls `plugins` on every
			// page load for every account to fetch the plugin JS/CSS list.
			// Restricting it breaks the app for everyone without MANAGE_PLUGINS.
			field: "plugins",
			want:  models.CapViewLibrary,
			why:   "the app-wide plugin loader calls this for every account on every page load",
		},
		{
			field: "pluginTasks",
			want:  models.CapManagePlugins,
			why:   "unlike `plugins`, this is only reached from Settings > Tasks",
		},
		{
			field: "directory",
			want:  models.CapBrowseFilesystem,
			why:   "walks arbitrary host filesystem paths — the most sensitive read in the schema",
		},
		{
			field: "findFiles",
			want:  models.CapBrowseFilesystem,
			why:   "returns real filesystem paths, which are not library content",
		},
		{
			field: "logs",
			want:  models.CapViewSystem,
			why:   "logs leak paths, config and request detail",
		},
		{
			field: "systemStatus",
			want:  models.CapViewSystem,
			why: "safe to restrict: the Setup and Migrate flows that call it run with " +
				"no credentials configured, or with the database not yet open, and both " +
				"of those already resolve to the admin preset",
		},
		{
			field: "findUsers",
			want:  models.CapManageUsers,
			why:   "other people's accounts are not library content",
		},
		{
			field: "scrapeURL",
			want:  models.CapScrape,
			why:   "makes the server fetch an arbitrary URL on the caller's behalf",
		},
		{
			field: "findShareLinks",
			want:  models.CapManageShares,
			why:   "matches the shareLink* mutations",
		},
		{
			field: "someQueryAddedLater",
			want:  models.CapViewLibrary,
			why: "an unclassified query defaults to readable — deliberate, so adding a " +
				"content query cannot accidentally lock out READ_ONLY. A new query that " +
				"exposes the host must be listed in queryCapabilities explicitly",
		},
	}

	for _, c := range cases {
		t.Run(c.field, func(t *testing.T) {
			if got := requiredCapabilityForQuery(c.field); got != c.want {
				t.Errorf("requiredCapabilityForQuery(%q) = %q, want %q\nwhy this matters: %s",
					c.field, got, c.want, c.why)
			}
		})
	}
}

// TestLegacyRoleBehaviourPreserved is the equivalence check for the capability
// migration at the operation level: for a representative set of operations,
// each role preset must allow exactly what that role allowed before.
//
// pkg/models tests the presets in isolation; this checks the mapping wired to
// them, which is where a typo in queryCapabilities or mutationCapabilities
// would actually bite.
func TestLegacyRoleBehaviourPreserved(t *testing.T) {
	type op struct {
		field      string
		isMutation bool
	}

	// field -> whether each role could perform it before capabilities existed
	expected := map[op]map[models.UserRole]bool{
		{"sceneAddO", true}: {
			models.UserRoleReadOnly: true, models.UserRoleUser: true, models.UserRoleAdmin: true,
		},
		{"changePassword", true}: {
			models.UserRoleReadOnly: true, models.UserRoleUser: true, models.UserRoleAdmin: true,
		},
		{"sceneUpdate", true}: {
			models.UserRoleReadOnly: false, models.UserRoleUser: true, models.UserRoleAdmin: true,
		},
		{"sceneDestroy", true}: {
			models.UserRoleReadOnly: false, models.UserRoleUser: false, models.UserRoleAdmin: true,
		},
		{"configureGeneral", true}: {
			models.UserRoleReadOnly: false, models.UserRoleUser: false, models.UserRoleAdmin: true,
		},
		{"userCreate", true}: {
			models.UserRoleReadOnly: false, models.UserRoleUser: false, models.UserRoleAdmin: true,
		},
		// DELIBERATE DEPARTURE: USER could create share links before capabilities
		// existed. Share links bypass authentication entirely (routes_share.go),
		// so publishing one is now admin-by-default and granted per account.
		// This is the only entry here that does not match the legacy behaviour.
		{"shareLinkCreate", true}: {
			models.UserRoleReadOnly: false, models.UserRoleUser: false, models.UserRoleAdmin: true,
		},
		{"findScenes", false}: {
			models.UserRoleReadOnly: true, models.UserRoleUser: true, models.UserRoleAdmin: true,
		},
		{"plugins", false}: {
			models.UserRoleReadOnly: true, models.UserRoleUser: true, models.UserRoleAdmin: true,
		},
		{"directory", false}: {
			models.UserRoleReadOnly: false, models.UserRoleUser: false, models.UserRoleAdmin: true,
		},
		{"scrapeURL", false}: {
			models.UserRoleReadOnly: false, models.UserRoleUser: true, models.UserRoleAdmin: true,
		},
	}

	for o, byRole := range expected {
		for role, allowed := range byRole {
			t.Run(o.field+"/"+string(role), func(t *testing.T) {
				var required models.Capability
				if o.isMutation {
					required = requiredCapabilityForMutation(o.field)
				} else {
					required = requiredCapabilityForQuery(o.field)
				}

				got := models.CapabilitiesForRole(role).Has(required)
				if got != allowed {
					verb := "gained"
					if allowed {
						verb = "lost"
					}
					t.Errorf("%s on %q: %s access (needs %s). The capability presets must "+
						"reproduce the pre-capability role behaviour exactly, or accounts "+
						"silently change permissions on upgrade.", role, o.field, verb, required)
				}
			})
		}
	}
}

func TestMeQueryIsExempt(t *testing.T) {
	if !exemptQueries["me"] {
		t.Error("`me` must bypass the capability check so it can return null when logged out")
	}
}
