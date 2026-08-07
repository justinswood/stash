package migrations

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/sqlite"
)

// post85 copies the instance-wide UI configuration to the first admin account.
//
// This cannot be done in SQL: the blob lives in the config file, not the
// database. Without it the admin would open the site after upgrading and find
// their front page, default filters and theme settings replaced by stock
// defaults — which reads as data loss even though the file still holds them.
//
// Every other account is deliberately left with no row, so it gets Stash
// defaults instead of inheriting the admin's layout.
func post85(ctx context.Context, db *sqlx.DB) error {
	logger.Info("Running post-migration for schema version 85")

	c := config.GetInstance()
	if c == nil {
		// no config loaded (tests, or a fresh install with nothing to carry over)
		return nil
	}

	uiConfig := c.GetUIConfiguration()
	if len(uiConfig) == 0 {
		logger.Info("No instance UI configuration to migrate")
		return nil
	}

	var adminID int
	err := db.QueryRowxContext(ctx,
		"SELECT id FROM users WHERE role = 'ADMIN' ORDER BY id LIMIT 1").Scan(&adminID)
	if err != nil {
		// No admin exists — an instance with authentication disabled. There is
		// no account to attribute the configuration to, and no second account
		// to hide it from, so leaving the table empty is correct: the config
		// file remains the only source and nothing changes for that install.
		logger.Infof("No admin account found; leaving UI configuration instance-wide (%v)", err)
		return nil
	}

	encoded, err := json.Marshal(uiConfig)
	if err != nil {
		return fmt.Errorf("encoding UI configuration: %w", err)
	}

	if _, err := db.ExecContext(ctx,
		"INSERT INTO user_ui_config (user_id, config) VALUES (?, ?)",
		adminID, string(encoded)); err != nil {
		return fmt.Errorf("copying UI configuration to user %d: %w", adminID, err)
	}

	logger.Infof("Copied instance UI configuration to admin account %d", adminID)
	return nil
}

func init() {
	sqlite.RegisterPostMigration(85, post85)
}
