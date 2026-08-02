package api

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/session"
	"github.com/stashapp/stash/pkg/sqlite"
)

const (
	tripwireActivatedErrMsg = "Stash is exposed to the public internet without authentication, and is not serving any more content to protect your privacy. " +
		"More information and fixes are available at https://discourse.stashapp.cc/t/-/1658"

	externalAccessErrMsg = "You have attempted to access Stash over the internet, and authentication is not enabled. " +
		"This is extremely dangerous! The whole world can see your your stash page and browse your files! " +
		"Stash is not answering any other requests to protect your privacy. " +
		"Please read the log entry or visit https://discourse.stashapp.cc/t/-/1658"
)

func allowUnauthenticated(r *http.Request) bool {
	// #2715 - allow access to UI files
	// /theme is the bundled default theme's static assets (fonts, images) — the
	// login page (served pre-auth) needs the theme fonts to render.
	if strings.HasPrefix(r.URL.Path, loginEndpoint) || r.URL.Path == logoutEndpoint || r.URL.Path == "/css" || strings.HasPrefix(r.URL.Path, "/assets") || strings.HasPrefix(r.URL.Path, "/theme") {
		return true
	}
	// Share links use their own per-token auth (see routes_share.go).
	// They are public by design — the token is the credential.
	if IsShareTokenPath(r.URL.Path) {
		return true
	}
	return false
}

func authenticateHandler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := config.GetInstance()

			// Share links are public by design (the token is the credential)
			// and must bypass tripwire / public-access enforcement so that
			// recipients on the open internet can use them.
			if IsShareTokenPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// error if external access tripwire activated
			if accessErr := session.CheckExternalAccessTripwire(c); accessErr != nil {
				http.Error(w, tripwireActivatedErrMsg, http.StatusForbidden)
				return
			}

			userID, err := manager.GetInstance().SessionStore.Authenticate(w, r)
			if err != nil {
				if !errors.Is(err, session.ErrUnauthorized) {
					logger.Errorf("Authentication error: %v", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}

				// unauthorized error
				w.Header().Add("WWW-Authenticate", "FormBased")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if err := session.CheckAllowPublicWithoutAuth(c, r); err != nil {
				var accessErr session.ExternalAccessError
				if errors.As(err, &accessErr) {
					session.LogExternalAccessError(accessErr)

					err := c.ActivatePublicAccessTripwire(net.IP(accessErr).String())
					if err != nil {
						logger.Errorf("Error activating public access tripwire: %v", err)
					}

					http.Error(w, externalAccessErrMsg, http.StatusForbidden)
				} else {
					logger.Errorf("Error checking external access security: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
				}
				return
			}

			ctx := r.Context()

			if c.HasCredentials() {
				// authentication is required
				if userID == "" && !allowUnauthenticated(r) {
					// if graphql or a non-webpage was requested, we just return a forbidden error
					ext := path.Ext(r.URL.Path)
					if r.URL.Path == gqlEndpoint || (ext != "" && ext != ".html") {
						w.Header().Add("WWW-Authenticate", "FormBased")
						w.WriteHeader(http.StatusUnauthorized)
						return
					}

					prefix := getProxyPrefix(r)

					// otherwise redirect to the login page
					returnURL := url.URL{
						Path:     prefix + r.URL.Path,
						RawQuery: r.URL.RawQuery,
					}
					q := make(url.Values)
					q.Set(returnURLParam, returnURL.String())
					u := url.URL{
						Path:     prefix + loginEndpoint,
						RawQuery: q.Encode(),
					}
					http.Redirect(w, r, u.String(), http.StatusFound)
					return
				}
			}

			// Reject a disabled account's surviving session. Disabled was
			// previously only checked at login, so an account disabled while
			// logged in kept full access — including media streaming — until
			// its cookie expired. Checked here rather than only in the GraphQL
			// layer so it covers the media routes too; backed by an in-memory
			// set so it costs no database lookup per asset.
			if userID != "" && manager.GetInstance().IsUserDisabled(userID) {
				logger.Warnf("rejecting request for disabled account %q", userID)
				if err := manager.GetInstance().SessionStore.Logout(w, r); err != nil {
					logger.Errorf("error clearing session for disabled account: %v", err)
				}
				w.Header().Add("WWW-Authenticate", "FormBased")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			ctx = session.SetCurrentUserID(ctx, userID)

			// Scope what this request may see, on every path including media
			// routes — otherwise a restricted account could still fetch hidden
			// content by its id. Served from an in-memory cache so it costs no
			// database read per asset. Never applied when userID is empty, so
			// background tasks and unauthenticated paths stay unscoped.
			if userID != "" {
				if cr := manager.GetInstance().ContentRestrictionsForUsername(userID); !cr.Empty() {
					ctx = sqlite.WithContentRestrictions(ctx, cr)
				}
			}

			// resolve the account id and attach it so per-user view/o history
			// and resume position are scoped to this user. Done here (before the
			// dataloaders middleware) so the request-scoped loaders capture it.
			// Only for the GraphQL endpoint to avoid a lookup on every asset.
			if userID != "" && r.URL.Path == gqlEndpoint {
				if u, uErr := manager.GetInstance().GetUserByUsername(ctx, userID); uErr == nil && u != nil {
					ctx = sqlite.WithHistoryUser(ctx, u.ID)
					// share this lookup with the permission checks (getCurrentUser)
					// so they don't re-query the same account per request.
					ctx = withCurrentUser(ctx, u)

					// resolve capabilities once per request rather than per root
					// field. On failure we attach nothing and currentCapabilities
					// recomputes — it must not fall back to the role preset here,
					// or a revoked capability would silently come back.
					if o, oErr := manager.GetInstance().GetUserCapabilityOverrides(ctx, u.ID); oErr == nil {
						ctx = withCurrentCapabilities(ctx, models.EffectiveCapabilities(u.Role, u.Disabled, o))
					} else {
						logger.Errorf("error loading capability overrides for %q: %v", userID, oErr)
					}

				}
			}

			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
