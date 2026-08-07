//go:build integration
// +build integration

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/stashapp/stash/pkg/sqlite"
)

// These tests exist because the guard they check was missing from the
// performer route for the lifetime of the content-scoping feature. Query
// filters hid the performer from every list, and the findPerformer resolver
// returned null, but GET /performer/{id}/image returned the photo with HTTP
// 200 to an account restricted from seeing it. Two of three enforcement layers
// held and the third was open, which is invisible unless the raw media URL is
// requested directly.
//
// Each route type therefore gets the same three assertions: hidden content is
// refused, visible content is not, and an unrestricted request is unaffected.
// The last one matters as much as the first — a guard that refuses everything
// would pass a test that only checks the hidden case, and would break the
// library for every ordinary user.

// sentinelHandler reports that the request reached the handler behind the
// middleware. If a Ctx middleware refuses the request this never runs, which is
// exactly the distinction under test.
func sentinelHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("reached handler"))
	})
}

// serveWithParam runs a Ctx middleware with the given chi URL parameter, under
// the supplied context, and reports the resulting status code.
func serveWithParam(mw func(http.Handler) http.Handler, paramKey string, paramValue string, ctx context.Context) int {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(paramKey, paramValue)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	mw(sentinelHandler()).ServeHTTP(w, r)

	return w.Code
}

// restrictedCtx is a request context carrying content restrictions, as
// authenticateHandler attaches for a user in a restricting group.
func restrictedCtx() context.Context {
	return sqlite.WithContentRestrictions(context.Background(), testRestrictions())
}

func TestPerformerRouteScoping(t *testing.T) {
	rs := performerRoutes{
		routes:          routes{txnManager: db},
		performerFinder: db.Performer,
		sfwConfig:       stubSFWConfig{},
	}

	tests := []struct {
		name        string
		performerID int
		ctx         context.Context
		want        int
		why         string
	}{
		{
			name:        "hidden by gender is refused",
			performerID: performerHiddenGenderID,
			ctx:         restrictedCtx(),
			want:        http.StatusNotFound,
			why:         "a restricted account must not fetch the photo of a performer it cannot see",
		},
		{
			name:        "hidden by tag is refused",
			performerID: performerHiddenTagID,
			ctx:         restrictedCtx(),
			want:        http.StatusNotFound,
			why:         "tag exclusions must hide the media route too, not only list queries",
		},
		{
			name:        "visible performer is served",
			performerID: performerVisibleID,
			ctx:         restrictedCtx(),
			want:        http.StatusOK,
			why:         "a guard that refuses permitted content would break the library for restricted users",
		},
		{
			name:        "unrestricted request is unaffected",
			performerID: performerHiddenGenderID,
			ctx:         context.Background(),
			want:        http.StatusOK,
			why:         "admins and background tasks carry no restrictions and must see everything",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := serveWithParam(rs.PerformerCtx, "performerId", itoa(tt.performerID), tt.ctx)
			if got != tt.want {
				t.Errorf("PerformerCtx = %d, want %d — %s", got, tt.want, tt.why)
			}
		})
	}
}

func TestImageRouteScoping(t *testing.T) {
	rs := imageRoutes{
		routes:      routes{txnManager: db},
		imageFinder: db.Image,
		fileGetter:  db.File,
	}

	tests := []struct {
		name    string
		imageID int
		ctx     context.Context
		want    int
		why     string
	}{
		{
			name:    "hidden image is refused",
			imageID: imageHiddenID,
			ctx:     restrictedCtx(),
			want:    http.StatusNotFound,
			why:     "an image featuring an excluded performer must not be reachable by id",
		},
		{
			name:    "visible image is served",
			imageID: imageVisibleID,
			ctx:     restrictedCtx(),
			want:    http.StatusOK,
			why:     "permitted images must still be served to a restricted account",
		},
		{
			name:    "unrestricted request is unaffected",
			imageID: imageHiddenID,
			ctx:     context.Background(),
			want:    http.StatusOK,
			why:     "no restrictions means the whole library stays reachable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := serveWithParam(rs.ImageCtx, "imageId", itoa(tt.imageID), tt.ctx)
			if got != tt.want {
				t.Errorf("ImageCtx = %d, want %d — %s", got, tt.want, tt.why)
			}
		})
	}
}

func TestGalleryRouteScoping(t *testing.T) {
	rs := galleryRoutes{
		routes:        routes{txnManager: db},
		galleryFinder: db.Gallery,
		imageFinder:   db.Image,
		fileGetter:    db.File,
	}

	tests := []struct {
		name      string
		galleryID int
		ctx       context.Context
		want      int
		why       string
	}{
		{
			name:      "hidden gallery is refused",
			galleryID: galleryHiddenID,
			ctx:       restrictedCtx(),
			want:      http.StatusNotFound,
			why:       "a gallery featuring an excluded performer must not be reachable by id",
		},
		{
			name:      "visible gallery is served",
			galleryID: galleryVisibleID,
			ctx:       restrictedCtx(),
			want:      http.StatusOK,
			why:       "permitted galleries must still be served to a restricted account",
		},
		{
			name:      "unrestricted request is unaffected",
			galleryID: galleryHiddenID,
			ctx:       context.Background(),
			want:      http.StatusOK,
			why:       "no restrictions means the whole library stays reachable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := serveWithParam(rs.GalleryCtx, "galleryId", itoa(tt.galleryID), tt.ctx)
			if got != tt.want {
				t.Errorf("GalleryCtx = %d, want %d — %s", got, tt.want, tt.why)
			}
		})
	}
}

func TestSceneRouteScoping(t *testing.T) {
	rs := sceneRoutes{
		routes:      routes{txnManager: db},
		sceneFinder: db.Scene,
		fileGetter:  db.File,
	}

	tests := []struct {
		name    string
		sceneID int
		ctx     context.Context
		want    int
		why     string
	}{
		{
			name:    "hidden scene is refused",
			sceneID: sceneHiddenID,
			ctx:     restrictedCtx(),
			want:    http.StatusNotFound,
			why:     "a scene featuring an excluded performer must not be reachable by id",
		},
		{
			name:    "visible scene is served",
			sceneID: sceneVisibleID,
			ctx:     restrictedCtx(),
			want:    http.StatusOK,
			why:     "permitted scenes must still be served to a restricted account",
		},
		{
			name:    "unrestricted request is unaffected",
			sceneID: sceneHiddenID,
			ctx:     context.Background(),
			want:    http.StatusOK,
			why:     "no restrictions means the whole library stays reachable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := serveWithParam(rs.SceneCtx, "sceneId", itoa(tt.sceneID), tt.ctx)
			if got != tt.want {
				t.Errorf("SceneCtx = %d, want %d — %s", got, tt.want, tt.why)
			}
		})
	}
}

// stubSFWConfig satisfies the performer route's config dependency. The value is
// irrelevant to scoping — it only selects which placeholder image is served
// when a performer has no photo.
type stubSFWConfig struct{}

func (stubSFWConfig) GetSFWContentMode() bool { return false }

func itoa(i int) string {
	return strconv.Itoa(i)
}
