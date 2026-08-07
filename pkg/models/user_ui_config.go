package models

import "context"

// UserUIConfigReader reads an account's interface configuration. A nil map
// means the account has never customised anything, which is distinct from an
// account that has deliberately cleared its settings (empty map).
type UserUIConfigReader interface {
	Get(ctx context.Context, userID int) (map[string]interface{}, error)
}

type UserUIConfigWriter interface {
	Set(ctx context.Context, userID int, cfg map[string]interface{}) error
}

type UserUIConfigReaderWriter interface {
	UserUIConfigReader
	UserUIConfigWriter
}
