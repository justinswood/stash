package models

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
)

// Capability is a single permission. Capabilities are what the API actually
// checks; roles are presets over them (see CapabilitiesForRole).
//
// The set is deliberately coarse — one capability per kind of thing a person
// might reasonably be trusted with separately. Splitting further produces
// combinations nobody configures, and every capability has to be mapped to
// every operation in permissions_middleware.go to mean anything.
type Capability string

const (
	// --- READ_ONLY preset ---

	// CapViewLibrary is the ability to browse scenes, performers, tags and the
	// rest of the collection. Every role has it; without it an account can log
	// in and see nothing.
	CapViewLibrary Capability = "VIEW_LIBRARY"
	// CapOwnHistory is recording one's own view and o history.
	CapOwnHistory Capability = "OWN_HISTORY"
	// CapChangeOwnPassword is rotating one's own password. Held by every role so
	// that a compromised account can always be secured by its owner.
	CapChangeOwnPassword Capability = "CHANGE_OWN_PASSWORD"

	// --- added by the USER preset ---

	// CapEditMetadata is creating and updating library metadata: scenes,
	// performers, studios, tags, galleries, groups and markers.
	CapEditMetadata Capability = "EDIT_METADATA"
	// CapScrape is running scrapers. Separate from CapEditMetadata because
	// scraping makes the server fetch arbitrary external URLs on the caller's
	// behalf, which is a different kind of trust.
	CapScrape Capability = "SCRAPE"
	// CapManageShares is creating, updating and revoking share links. Separate
	// because a share link publishes content to the open internet with no
	// authentication — see routes_share.go.
	CapManageShares Capability = "MANAGE_SHARES"
	// CapManageOwnAPIKeys is issuing and revoking one's own API keys.
	CapManageOwnAPIKeys Capability = "MANAGE_OWN_API_KEYS"

	// --- added by the ADMIN preset ---

	// CapDeleteContent is destroying library objects and files.
	CapDeleteContent Capability = "DELETE_CONTENT"
	// CapManageUsers is creating, updating and deleting accounts, and reading
	// other people's accounts and API keys.
	CapManageUsers Capability = "MANAGE_USERS"
	// CapConfigure is changing instance configuration.
	CapConfigure Capability = "CONFIGURE"
	// CapRunTasks is running library tasks: scan, generate, identify, clean,
	// import/export, migrate and backup.
	CapRunTasks Capability = "RUN_TASKS"
	// CapViewSystem is reading operational state: logs, the job queue, system
	// status and DLNA status.
	CapViewSystem Capability = "VIEW_SYSTEM"
	// CapBrowseFilesystem is enumerating paths on the host, which is not the
	// same as browsing the library.
	CapBrowseFilesystem Capability = "BROWSE_FILESYSTEM"
	// CapManagePlugins is installing, enabling and running plugins and packages.
	CapManagePlugins Capability = "MANAGE_PLUGINS"
	// CapExecuteSQL is running arbitrary SQL against the database. Held only by
	// admins and worth revoking even from most of them.
	CapExecuteSQL Capability = "EXECUTE_SQL"
)

// AllCapabilities is every capability, in preset order. Used to validate input
// and to render the management UI.
var AllCapabilities = []Capability{
	CapViewLibrary,
	CapOwnHistory,
	CapChangeOwnPassword,
	CapEditMetadata,
	CapScrape,
	CapManageShares,
	CapManageOwnAPIKeys,
	CapDeleteContent,
	CapManageUsers,
	CapConfigure,
	CapRunTasks,
	CapViewSystem,
	CapBrowseFilesystem,
	CapManagePlugins,
	CapExecuteSQL,
}

func (c Capability) IsValid() bool {
	for _, v := range AllCapabilities {
		if v == c {
			return true
		}
	}
	return false
}

func (c Capability) String() string { return string(c) }

func (c *Capability) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}
	*c = Capability(str)
	if !c.IsValid() {
		return fmt.Errorf("%s is not a valid Capability", str)
	}
	return nil
}

func (c Capability) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(c.String()))
}

// CapabilitySet is a set of capabilities.
type CapabilitySet map[Capability]struct{}

func (s CapabilitySet) Has(c Capability) bool {
	_, ok := s[c]
	return ok
}

// Sorted returns the set in AllCapabilities order, for stable API output.
func (s CapabilitySet) Sorted() []Capability {
	ret := make([]Capability, 0, len(s))
	for _, c := range AllCapabilities {
		if s.Has(c) {
			ret = append(ret, c)
		}
	}
	return ret
}

var (
	readOnlyCaps = []Capability{
		CapViewLibrary,
		CapOwnHistory,
		CapChangeOwnPassword,
	}
	userCaps = append(append([]Capability{}, readOnlyCaps...),
		CapEditMetadata,
		CapScrape,
		CapManageShares,
		CapManageOwnAPIKeys,
	)
	adminCaps = append(append([]Capability{}, userCaps...),
		CapDeleteContent,
		CapManageUsers,
		CapConfigure,
		CapRunTasks,
		CapViewSystem,
		CapBrowseFilesystem,
		CapManagePlugins,
		CapExecuteSQL,
	)
)

// CapabilitiesForRole returns the preset capability set for a role. The presets
// reproduce exactly what the three roles could do before capabilities existed,
// so an instance with no per-user overrides behaves identically.
func CapabilitiesForRole(r UserRole) CapabilitySet {
	var list []Capability
	switch r {
	case UserRoleAdmin:
		list = adminCaps
	case UserRoleUser:
		list = userCaps
	case UserRoleReadOnly:
		list = readOnlyCaps
	default:
		return CapabilitySet{}
	}

	set := make(CapabilitySet, len(list))
	for _, c := range list {
		set[c] = struct{}{}
	}
	return set
}

// UserCapabilityOverride is a per-user departure from the role preset. Granted
// reports whether the capability is added (true) or removed (false).
type UserCapabilityOverride struct {
	UserID     int        `db:"user_id" json:"user_id"`
	Capability Capability `db:"capability" json:"capability"`
	Granted    bool       `db:"granted" json:"granted"`
}

// EffectiveCapabilities applies a user's overrides to their role preset:
//
//	effective = preset(role) + granted - revoked
//
// A disabled account resolves to the empty set regardless of role or overrides.
func EffectiveCapabilities(role UserRole, disabled bool, overrides []UserCapabilityOverride) CapabilitySet {
	if disabled {
		return CapabilitySet{}
	}

	set := CapabilitiesForRole(role)
	for _, o := range overrides {
		if !o.Capability.IsValid() {
			continue
		}
		if o.Granted {
			set[o.Capability] = struct{}{}
		} else {
			delete(set, o.Capability)
		}
	}
	return set
}

// SortOverrides orders overrides deterministically, for stable output.
func SortOverrides(o []UserCapabilityOverride) {
	sort.Slice(o, func(i, j int) bool {
		return string(o[i].Capability) < string(o[j].Capability)
	})
}

type UserCapabilityReader interface {
	FindByUserID(ctx context.Context, userID int) ([]UserCapabilityOverride, error)
}

type UserCapabilityWriter interface {
	// SetForUser replaces the complete override set for a user.
	SetForUser(ctx context.Context, userID int, overrides []UserCapabilityOverride) error
}

type UserCapabilityReaderWriter interface {
	UserCapabilityReader
	UserCapabilityWriter
}
