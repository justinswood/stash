package models

import "testing"

// TestPresetsReproduceLegacyRoleBehaviour is the load-bearing test of the whole
// capability migration. Roles used to be a single rank compared with AtLeast();
// they are now presets over capabilities. If a preset drifts from what the role
// could previously do, existing accounts silently gain or lose permissions on
// upgrade — with no migration to point at and nothing in the UI to show it.
func TestPresetsReproduceLegacyRoleBehaviour(t *testing.T) {
	// What each role could do before capabilities existed, taken from the old
	// readOnlyMutations / adminMutations classification.
	legacy := map[UserRole]struct {
		must    []Capability
		mustNot []Capability
	}{
		UserRoleReadOnly: {
			must: []Capability{
				CapViewLibrary, CapOwnHistory, CapChangeOwnPassword,

				// DELIBERATE DEPARTURE from the pre-capability behaviour. Saved
				// filters and UI configuration used to fall under "the rest
				// require USER", so a READ_ONLY account could not save a filter
				// or arrange its own front page.
				//
				// That was tolerable only while both were global: the account
				// inherited whatever the admin had set up. Migration 84 made
				// saved filters per-user and the UI configuration followed, so
				// withholding this now would leave a READ_ONLY account with no
				// filters, a default front page, and no way to change either.
				//
				// It grants nothing over the library — these mutations only
				// change what their own caller sees.
				CapOwnViewSettings,
			},
			mustNot: []Capability{
				CapEditMetadata, CapScrape, CapManageShares, CapManageOwnAPIKeys,
				CapDeleteContent, CapManageUsers, CapConfigure, CapRunTasks,
				CapViewSystem, CapBrowseFilesystem, CapManagePlugins, CapExecuteSQL,
			},
		},
		UserRoleUser: {
			must: []Capability{
				CapViewLibrary, CapOwnHistory, CapChangeOwnPassword,
				CapEditMetadata, CapScrape, CapManageOwnAPIKeys,
				// USER could already save filters under the old model; this
				// keeps that, it is only new for READ_ONLY.
				CapOwnViewSettings,
			},
			mustNot: []Capability{
				// the USER role never deleted content or touched the system
				CapDeleteContent, CapManageUsers, CapConfigure, CapRunTasks,
				CapViewSystem, CapBrowseFilesystem, CapManagePlugins, CapExecuteSQL,

				// DELIBERATE DEPARTURE from the pre-capability behaviour. The USER
				// role could previously create share links, because shareLink*
				// mutations were classified USER-level. That was too permissive: a
				// share link is served by routes_share.go, which bypasses
				// authentication and the tripwire entirely, so any USER could
				// publish a performer's whole catalogue to the open internet.
				//
				// This is the one intentional behaviour change in the capability
				// migration. Accounts that need it are granted MANAGE_SHARES
				// individually — which is exactly what per-user overrides are for.
				CapManageShares,
			},
		},
		UserRoleAdmin: {
			must:    AllCapabilities,
			mustNot: nil,
		},
	}

	for role, want := range legacy {
		t.Run(string(role), func(t *testing.T) {
			got := CapabilitiesForRole(role)
			for _, c := range want.must {
				if !got.Has(c) {
					t.Errorf("%s lost %s — accounts with this role would lose access they had before capabilities existed", role, c)
				}
			}
			for _, c := range want.mustNot {
				if got.Has(c) {
					t.Errorf("%s gained %s — accounts with this role would silently gain access on upgrade", role, c)
				}
			}
		})
	}
}

// TestRolePresetsAreNested checks the presets remain supersets of each other.
// The roles are still presented as a hierarchy in the UI and in UserRole.AtLeast;
// if USER ever held something ADMIN did not, that ordering would be a lie.
func TestRolePresetsAreNested(t *testing.T) {
	ro := CapabilitiesForRole(UserRoleReadOnly)
	user := CapabilitiesForRole(UserRoleUser)
	admin := CapabilitiesForRole(UserRoleAdmin)

	for c := range ro {
		if !user.Has(c) {
			t.Errorf("USER is missing %s which READ_ONLY has", c)
		}
	}
	for c := range user {
		if !admin.Has(c) {
			t.Errorf("ADMIN is missing %s which USER has", c)
		}
	}
}

func TestEffectiveCapabilities(t *testing.T) {
	t.Run("no overrides equals the preset", func(t *testing.T) {
		got := EffectiveCapabilities(UserRoleUser, false, nil)
		want := CapabilitiesForRole(UserRoleUser)
		if len(got) != len(want) {
			t.Fatalf("got %d capabilities, want %d", len(got), len(want))
		}
		for c := range want {
			if !got.Has(c) {
				t.Errorf("missing %s", c)
			}
		}
	})

	t.Run("grant adds a capability the role lacks", func(t *testing.T) {
		got := EffectiveCapabilities(UserRoleUser, false, []UserCapabilityOverride{
			{Capability: CapDeleteContent, Granted: true},
		})
		if !got.Has(CapDeleteContent) {
			t.Error("granted capability was not added")
		}
		if !got.Has(CapEditMetadata) {
			t.Error("granting one capability must not disturb the rest of the preset")
		}
	})

	t.Run("revoke removes a capability the role includes", func(t *testing.T) {
		got := EffectiveCapabilities(UserRoleAdmin, false, []UserCapabilityOverride{
			{Capability: CapExecuteSQL, Granted: false},
		})
		if got.Has(CapExecuteSQL) {
			t.Error("revoked capability was not removed")
		}
		if !got.Has(CapManageUsers) {
			t.Error("revoking one capability must not disturb the rest of the preset")
		}
	})

	t.Run("disabled account has nothing", func(t *testing.T) {
		// Belt and braces: authenticateHandler rejects disabled accounts before
		// this runs, but an admin whose capabilities still resolved would be a
		// severe failure, so the set is empty regardless of role or grants.
		got := EffectiveCapabilities(UserRoleAdmin, true, []UserCapabilityOverride{
			{Capability: CapExecuteSQL, Granted: true},
		})
		if len(got) != 0 {
			t.Errorf("disabled account resolved to %d capabilities, want 0", len(got))
		}
	})

	t.Run("unknown capability in overrides is ignored", func(t *testing.T) {
		// Guards forward compatibility: a row written by a newer version must not
		// corrupt the set or panic on an older one.
		got := EffectiveCapabilities(UserRoleUser, false, []UserCapabilityOverride{
			{Capability: Capability("NOT_A_REAL_CAPABILITY"), Granted: true},
		})
		want := CapabilitiesForRole(UserRoleUser)
		if len(got) != len(want) {
			t.Errorf("unknown capability changed the set: got %d, want %d", len(got), len(want))
		}
	})

	t.Run("unknown role has no capabilities", func(t *testing.T) {
		if got := EffectiveCapabilities(UserRole("BOGUS"), false, nil); len(got) != 0 {
			t.Errorf("unknown role resolved to %d capabilities, want 0", len(got))
		}
	})
}

func TestCapabilitySetSortedIsStable(t *testing.T) {
	set := CapabilitiesForRole(UserRoleAdmin)
	first := set.Sorted()
	for i := 0; i < 5; i++ {
		got := set.Sorted()
		if len(got) != len(first) {
			t.Fatal("Sorted() returned a different length across calls")
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("Sorted() is not stable: position %d was %s, now %s", j, first[j], got[j])
			}
		}
	}
}
