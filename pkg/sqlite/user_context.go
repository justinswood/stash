package sqlite

import "context"

type historyUserKey struct{}

// WithHistoryUser returns a context carrying the current user's id, used to
// scope per-user view/o history and resume position. A userID of 0 means "no
// specific user" — history queries then aggregate across all users, preserving
// legacy behaviour for background tasks and non-authenticated callers.
func WithHistoryUser(ctx context.Context, userID int) context.Context {
	return context.WithValue(ctx, historyUserKey{}, userID)
}

// historyUserID returns the per-user history scope from context, or 0 if unset.
func historyUserID(ctx context.Context) int {
	if v, ok := ctx.Value(historyUserKey{}).(int); ok {
		return v
	}
	return 0
}
