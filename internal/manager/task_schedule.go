package manager

import (
	"context"
	"time"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/logger"
)

type scanScheduler struct {
	manager *Manager
	cancel  context.CancelFunc
}

func newScanScheduler(mgr *Manager) *scanScheduler {
	return &scanScheduler{manager: mgr}
}

func (s *scanScheduler) Start(cfg *config.Config) {
	s.Stop()

	schedule := cfg.GetScanSchedule()
	if schedule == "" {
		return
	}

	interval := parseScheduleInterval(schedule)
	if interval == 0 {
		logger.Warnf("Invalid scan_schedule value: %q", schedule)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	logger.Infof("Scan scheduler started: scanning every %s", interval)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Info("Scan scheduler stopped")
				return
			case <-ticker.C:
				s.runScan()
			}
		}
	}()
}

func (s *scanScheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

func (s *scanScheduler) runScan() {
	logger.Info("Scheduled scan starting...")

	input := ScanMetadataInput{}

	// Use default scan settings if configured
	cfg := config.GetInstance()
	if defaults := cfg.GetDefaultScanSettings(); defaults != nil {
		input.ScanMetadataOptions = *defaults
	}

	ctx := context.Background()
	if _, err := s.manager.Scan(ctx, input); err != nil {
		logger.Errorf("Scheduled scan failed: %v", err)
	}
}

func parseScheduleInterval(schedule string) time.Duration {
	switch schedule {
	case "hourly", "1h":
		return time.Hour
	case "6h":
		return 6 * time.Hour
	case "12h":
		return 12 * time.Hour
	case "daily", "24h":
		return 24 * time.Hour
	case "weekly":
		return 7 * 24 * time.Hour
	default:
		// Try parsing as a Go duration string
		d, err := time.ParseDuration(schedule)
		if err != nil {
			return 0
		}
		// Minimum 5 minutes to prevent abuse
		if d < 5*time.Minute {
			return 0
		}
		return d
	}
}
