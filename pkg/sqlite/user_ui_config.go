package sqlite

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	"gopkg.in/guregu/null.v4"
)

const userUIConfigTable = "user_ui_config"

// UserUIConfigStore holds each account's interface configuration: front page
// layout, default filters, theme settings and interface preferences.
//
// Before migration 85 this was a single blob in the instance config file, so
// every account saw the admin's front page. The instance blob still exists and
// is still the source for values the server reads outside a request (see
// migration 85), but what a browser is served is now per-account.
type UserUIConfigStore struct{}

func NewUserUIConfigStore() *UserUIConfigStore {
	return &UserUIConfigStore{}
}

// Get returns the stored configuration for a user, or nil when they have none.
//
// nil is meaningful and must not be conflated with an empty map: it means "this
// account has never customised anything", which the resolver answers with stock
// defaults. An account that has deliberately cleared its configuration stores an
// empty map and gets exactly that back.
func (qb *UserUIConfigStore) Get(ctx context.Context, userID int) (map[string]interface{}, error) {
	if userID <= 0 {
		return nil, nil
	}

	q := dialect.From(userUIConfigTable).
		Select("config").
		Where(goqu.C("user_id").Eq(userID)).
		Prepared(true)

	// Scanned as nullable rather than a plain string on purpose. querySimple
	// does NOT return sql.ErrNoRows when nothing matches — it leaves the output
	// untouched and returns nil (table.go). Against a plain string that makes
	// "this account has no row" indistinguishable from "this account stored an
	// empty config", and the caller needs to tell them apart: the first means
	// serve stock defaults, the second means the user cleared their settings on
	// purpose. The column is NOT NULL, so Valid is false only when no row
	// matched.
	var encoded null.String
	if err := querySimple(ctx, q, &encoded); err != nil {
		return nil, err
	}

	if !encoded.Valid {
		return nil, nil
	}

	if encoded.String == "" {
		return map[string]interface{}{}, nil
	}

	var ret map[string]interface{}
	if err := json.Unmarshal([]byte(encoded.String), &ret); err != nil {
		return nil, fmt.Errorf("decoding UI configuration for user %d: %w", userID, err)
	}

	return ret, nil
}

// Set replaces a user's configuration.
func (qb *UserUIConfigStore) Set(ctx context.Context, userID int, cfg map[string]interface{}) error {
	if userID <= 0 {
		// No user in context — an instance with authentication disabled. There
		// is no row to write; the caller falls back to the instance config file,
		// which is what such an install has always used.
		return nil
	}

	if cfg == nil {
		cfg = map[string]interface{}{}
	}

	encoded, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding UI configuration: %w", err)
	}

	// The primary key is user_id, so an upsert keeps this to one statement and
	// makes concurrent writes from two tabs idempotent rather than a conflict.
	q := dialect.Insert(userUIConfigTable).
		Prepared(true).
		Rows(goqu.Record{"user_id": userID, "config": string(encoded)}).
		OnConflict(goqu.DoUpdate("user_id", goqu.Record{"config": string(encoded)}))

	if _, err := exec(ctx, q); err != nil {
		return fmt.Errorf("setting UI configuration for user %d: %w", userID, err)
	}

	return nil
}
