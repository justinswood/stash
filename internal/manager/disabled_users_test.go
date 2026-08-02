package manager

import "testing"

// TestIsUserDisabled covers the in-memory set that authenticateHandler consults
// on every request. The behaviour that matters is the default: before
// RefreshDisabledUsers has ever run (startup, or a database that is not open
// yet) the set is nil, and a nil map must report "not disabled" rather than
// panicking or locking every account out of the instance.
func TestIsUserDisabled(t *testing.T) {
	m := &Manager{}

	t.Run("nil set denies nothing", func(t *testing.T) {
		if m.IsUserDisabled("anyone") {
			t.Error("an uninitialised set must not report accounts as disabled — " +
				"that would lock every user out before the first refresh")
		}
	})

	t.Run("empty username is never disabled", func(t *testing.T) {
		// An unauthenticated request carries userID "". It must fall through to
		// the normal unauthenticated handling, not be treated as a disabled user.
		if m.IsUserDisabled("") {
			t.Error("the empty username must not be treated as a disabled account")
		}
	})

	m.disabledUsers = map[string]struct{}{"banned": {}}

	t.Run("listed user is disabled", func(t *testing.T) {
		if !m.IsUserDisabled("banned") {
			t.Error("a user in the set must report as disabled")
		}
	})

	t.Run("unlisted user is not disabled", func(t *testing.T) {
		if m.IsUserDisabled("allowed") {
			t.Error("a user absent from the set must not report as disabled")
		}
	})

	t.Run("match is exact", func(t *testing.T) {
		// Guards against a future switch to prefix/fuzzy matching: "banned2"
		// must not inherit "banned"'s disabled state.
		if m.IsUserDisabled("banned2") {
			t.Error("username matching must be exact")
		}
	})
}
