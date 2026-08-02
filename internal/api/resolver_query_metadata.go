package api

import (
	"context"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/models"
)

// SystemStatus is readable by every account, because the app cannot boot
// without it: the Setup and Migrate flows use it to decide what to render, and
// it is fetched on every page load. Denying it leaves the UI spinning forever
// with no visible error.
//
// The host paths it carries are the part actually worth protecting, so those
// are blanked for accounts without VIEW_SYSTEM — the same redact-rather-than-
// deny approach resolver_query_configuration.go takes with the API key.
func (r *queryResolver) SystemStatus(ctx context.Context) (*manager.SystemStatus, error) {
	status := manager.GetInstance().GetSystemStatus()

	caps, err := r.currentCapabilities(ctx)
	if err != nil {
		return nil, err
	}
	if caps.Has(models.CapViewSystem) {
		return status, nil
	}

	// copy so the redaction cannot leak into the manager's shared instance
	redacted := *status
	redacted.DatabasePath = nil
	redacted.ConfigPath = nil
	redacted.FfmpegPath = nil
	redacted.FfprobePath = nil
	redacted.WorkingDir = ""
	redacted.HomeDir = ""
	return &redacted, nil
}
