package api

import (
	"context"
	"errors"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/session"
)

// ErrPermission is returned when the current user lacks the required role.
var ErrPermission = errors.New("insufficient permissions")

// withCurrentUser stashes the already-resolved account on the context so
// getCurrentUser can reuse it instead of re-querying. authenticateHandler
// resolves the user once per request (to scope history); this lets the
// permission checks share that lookup rather than hitting the DB again.
func withCurrentUser(ctx context.Context, u *models.User) context.Context {
	return context.WithValue(ctx, currentUserKey, u)
}

// getCurrentUser resolves the authenticated user's account, or nil when no user
// is logged in. When the session user matches the config credential but has no
// account row yet (break-glass admin, pre-bootstrap), a synthetic admin is
// returned.
func (r *Resolver) getCurrentUser(ctx context.Context) (*models.User, error) {
	uid := session.GetCurrentUserID(ctx)
	if uid == nil || *uid == "" {
		return nil, nil
	}

	// reuse the account resolved by authenticateHandler for this request, if any.
	if cached, ok := ctx.Value(currentUserKey).(*models.User); ok && cached != nil {
		return cached, nil
	}

	// if the database isn't open yet (e.g. a migration is pending) we can't look
	// up the account — treat the authenticated session as the break-glass admin
	// so that admin-only operations like `migrate` can still run.
	if db := manager.GetInstance().Database; db == nil || db.Ready() != nil {
		return &models.User{Username: *uid, Role: models.UserRoleAdmin}, nil
	}

	var u *models.User
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		u, err = r.repository.User.FindByUsername(ctx, *uid)
		return err
	}); err != nil {
		return nil, err
	}

	if u == nil {
		// The session (or config API key) names an account that does not exist.
		// Previously this returned a synthetic admin, which made the single
		// config API key an unconditional admin credential regardless of the
		// users table. Deny instead, and say so — a silent lockout here is very
		// hard to diagnose from the UI.
		logger.Warnf("authenticated principal %q has no user account; denying. "+
			"An admin account is normally seeded from the config credential at "+
			"startup (EnsureBootstrapAdmin).", *uid)
		return nil, nil
	}
	return u, nil
}

// currentRole returns the effective role for the request. When the system has no
// credentials configured at all it is open, so admin is returned to preserve the
// pre-multiuser experience.
func (r *Resolver) currentRole(ctx context.Context) (models.UserRole, error) {
	u, err := r.getCurrentUser(ctx)
	if err != nil {
		return "", err
	}
	if u == nil {
		if !manager.GetInstance().Config.HasCredentials() {
			return models.UserRoleAdmin, nil
		}
		return "", nil
	}
	return u.Role, nil
}

// requireRole returns ErrPermission unless the current user's role is at least
// the given minimum.
func (r *Resolver) requireRole(ctx context.Context, min models.UserRole) error {
	role, err := r.currentRole(ctx)
	if err != nil {
		return err
	}
	if role == "" || !role.AtLeast(min) {
		return ErrPermission
	}
	return nil
}

func (r *Resolver) requireAdmin(ctx context.Context) error {
	return r.requireRole(ctx, models.UserRoleAdmin)
}
