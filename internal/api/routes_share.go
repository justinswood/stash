package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/txn"
	"github.com/stashapp/stash/pkg/utils"
)

// shareRoutes wires up the public, token-authenticated `/share/<token>/...`
// subtree. All routes under this subtree pass through ShareCtx, which resolves
// the token to a ShareLink and enforces revoked/expired/limit checks.
type shareRoutes struct {
	routes
	shareLinkFinder   models.ShareLinkReaderWriter
	sceneFinder       SceneFinder
	imageFinder       ImageFinder
	performerFinder   PerformerFinder
	sfwConfig         sfwConfig
	fileGetter        models.FileGetter
	captionFinder     CaptionFinder
	sceneMarkerFinder SceneMarkerFinder
	tagFinder         SceneMarkerTagFinder
	scenePerformerIDs models.PerformerIDLoader
	imagePerformerIDs models.PerformerIDLoader
}

func (rs shareRoutes) Routes() chi.Router {
	r := chi.NewRouter()

	r.Route("/{token}", func(r chi.Router) {
		r.Use(rs.ShareCtx)

		// Index page: HTML viewer + view-count increment.
		r.Get("/", rs.Index)
		r.Get("/manifest.json", rs.Manifest)

		// Performer profile image (only valid for PERFORMER share type).
		r.Get("/performer/image", rs.PerformerImage)

		// Scene asset proxy. RequireScene asserts the sceneId is in the share's
		// allowlist; SceneCtx then loads the scene into the request context so the
		// reused stream handlers can find it.
		scene := rs.sceneSubRouter()
		r.Route("/scene/{sceneId}", func(r chi.Router) {
			r.Use(rs.RequireScene)
			r.Use(scene.SceneCtx)
			r.Get("/", rs.SceneViewer)
			r.Get("/stream", scene.StreamDirect)
			r.Get("/stream.mp4", scene.StreamMp4)
			r.Get("/stream.webm", scene.StreamWebM)
			r.Get("/stream.mkv", scene.StreamMKV)
			r.Get("/stream.m3u8", scene.StreamHLS)
			r.Get("/stream.m3u8/{segment}.ts", scene.StreamHLSSegment)
			r.Get("/stream.mpd", scene.StreamDASH)
			r.Get("/stream.mpd/{segment}_v.webm", scene.StreamDASHVideoSegment)
			r.Get("/stream.mpd/{segment}_a.webm", scene.StreamDASHAudioSegment)
			r.Get("/screenshot", scene.Screenshot)
			r.Get("/preview", scene.Preview)
			r.Get("/webp", scene.Webp)
		})

		// Image asset proxy.
		image := rs.imageSubRouter()
		r.Route("/image/{imageId}", func(r chi.Router) {
			r.Use(rs.RequireImage)
			r.Use(image.ImageCtx)
			r.Get("/", rs.ImageViewer)
			r.Get("/image", image.Image)
			r.Get("/thumbnail", image.Thumbnail)
			r.Get("/preview", image.Preview)
		})
	})

	return r
}

func (rs shareRoutes) sceneSubRouter() sceneRoutes {
	return sceneRoutes{
		routes:            rs.routes,
		sceneFinder:       rs.sceneFinder,
		fileGetter:        rs.fileGetter,
		captionFinder:     rs.captionFinder,
		sceneMarkerFinder: rs.sceneMarkerFinder,
		tagFinder:         rs.tagFinder,
	}
}

func (rs shareRoutes) imageSubRouter() imageRoutes {
	return imageRoutes{
		routes:      rs.routes,
		imageFinder: rs.imageFinder,
		fileGetter:  rs.fileGetter,
	}
}

// ShareCtx loads the share link from the URL token and validates it.
func (rs shareRoutes) ShareCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")
		if token == "" || !looksLikeShareToken(token) {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		tokenHash := HashShareToken(token)

		var link *models.ShareLink
		_ = rs.withReadTxn(r, func(ctx context.Context) error {
			var err error
			link, err = rs.shareLinkFinder.FindByTokenHash(ctx, tokenHash)
			return err
		})
		if link == nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		if link.Revoked {
			http.Error(w, "share link has been revoked", http.StatusGone)
			return
		}
		if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
			http.Error(w, "share link has expired", http.StatusGone)
			return
		}
		if link.ViewLimit != nil && link.ViewCount >= *link.ViewLimit {
			http.Error(w, "share link view limit reached", http.StatusGone)
			return
		}

		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")

		ctx := context.WithValue(r.Context(), shareKey, link)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireScene asserts that the share grants access to the requested {sceneId}.
// For SCENE shares, the sceneId must equal the share's scene_id. For PERFORMER
// shares, the scene must list the share's performer_id among its performers.
func (rs shareRoutes) RequireScene(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		link, ok := r.Context().Value(shareKey).(*models.ShareLink)
		if !ok || link == nil {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		sceneID, err := strconv.Atoi(chi.URLParam(r, "sceneId"))
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		switch link.ShareType {
		case models.ShareTypeScene:
			if link.SceneID == nil || *link.SceneID != sceneID {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
		case models.ShareTypePerformer:
			if link.PerformerID == nil {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			if !rs.sceneBelongsToPerformer(r, sceneID, *link.PerformerID) {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
		default:
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireImage asserts that the share grants access to the requested {imageId}.
func (rs shareRoutes) RequireImage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		link, ok := r.Context().Value(shareKey).(*models.ShareLink)
		if !ok || link == nil {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		imageID, err := strconv.Atoi(chi.URLParam(r, "imageId"))
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		switch link.ShareType {
		case models.ShareTypeImage:
			if link.ImageID == nil || *link.ImageID != imageID {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
		case models.ShareTypePerformer:
			if link.PerformerID == nil {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			if !rs.imageBelongsToPerformer(r, imageID, *link.PerformerID) {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
		default:
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rs shareRoutes) sceneBelongsToPerformer(r *http.Request, sceneID, performerID int) bool {
	var ok bool
	_ = rs.withReadTxn(r, func(ctx context.Context) error {
		ids, err := rs.scenePerformerIDs.GetPerformerIDs(ctx, sceneID)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if id == performerID {
				ok = true
				return nil
			}
		}
		return nil
	})
	return ok
}

func (rs shareRoutes) imageBelongsToPerformer(r *http.Request, imageID, performerID int) bool {
	var ok bool
	_ = rs.withReadTxn(r, func(ctx context.Context) error {
		ids, err := rs.imagePerformerIDs.GetPerformerIDs(ctx, imageID)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if id == performerID {
				ok = true
				return nil
			}
		}
		return nil
	})
	return ok
}

// Index renders the share landing page and increments the view count.
func (rs shareRoutes) Index(w http.ResponseWriter, r *http.Request) {
	link := r.Context().Value(shareKey).(*models.ShareLink)

	if err := rs.withTxnPlain(r, func(ctx context.Context) error {
		_, err := rs.shareLinkFinder.IncrementViewCount(ctx, link.ID)
		return err
	}); err != nil {
		http.Error(w, "share link view limit reached", http.StatusGone)
		return
	}

	switch link.ShareType {
	case models.ShareTypeScene:
		rs.renderSceneViewerStandalone(w, r, link)
	case models.ShareTypePerformer:
		rs.renderPerformerViewer(w, r, link)
	default:
		http.Error(w, "share type not yet supported", http.StatusNotImplemented)
	}
}

// SceneViewer renders the scene player (used both as the SCENE-share index and
// as the per-scene page within a PERFORMER share).
func (rs shareRoutes) SceneViewer(w http.ResponseWriter, r *http.Request) {
	link := r.Context().Value(shareKey).(*models.ShareLink)
	scene := r.Context().Value(sceneKey).(*models.Scene)
	rs.renderSceneViewerForScene(w, r, link, scene)
}

// ImageViewer renders a fullsize image with a back link.
func (rs shareRoutes) ImageViewer(w http.ResponseWriter, r *http.Request) {
	link := r.Context().Value(shareKey).(*models.ShareLink)
	img := r.Context().Value(imageKey).(*models.Image)

	token := chi.URLParam(r, "token")
	title := img.GetTitle()
	if title == "" {
		title = fmt.Sprintf("Image #%d", img.ID)
	}

	backHref := ""
	if link.ShareType == models.ShareTypePerformer {
		backHref = "/share/" + token + "/"
	}

	data := imageViewerData{
		Title:    title,
		ImageURL: fmt.Sprintf("/share/%s/image/%d/image", token, img.ID),
		BackHref: backHref,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", shareViewerCSP)
	w.Header().Set("X-Frame-Options", "DENY")
	if err := imageViewerTemplate.Execute(w, data); err != nil {
		logger.Errorf("[share] error rendering image viewer: %v", err)
	}
}

// PerformerImage serves the performer's profile image. Only valid for
// PERFORMER share type.
func (rs shareRoutes) PerformerImage(w http.ResponseWriter, r *http.Request) {
	link := r.Context().Value(shareKey).(*models.ShareLink)
	if link.ShareType != models.ShareTypePerformer || link.PerformerID == nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	var performer *models.Performer
	var image []byte
	_ = rs.withReadTxn(r, func(ctx context.Context) error {
		var err error
		performer, err = rs.performerFinder.Find(ctx, *link.PerformerID)
		if err != nil || performer == nil {
			return err
		}
		image, err = rs.performerFinder.GetImage(ctx, performer.ID)
		return err
	})
	if performer == nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if len(image) == 0 {
		image = getDefaultPerformerImage(performer.Name, performer.Gender, rs.sfwConfig.GetSFWContentMode())
	}
	utils.ServeImage(w, r, image)
}

// Manifest returns a small JSON description of the share.
func (rs shareRoutes) Manifest(w http.ResponseWriter, r *http.Request) {
	link := r.Context().Value(shareKey).(*models.ShareLink)
	token := chi.URLParam(r, "token")

	out := manifestResponse{
		ShareType: string(link.ShareType),
	}

	switch link.ShareType {
	case models.ShareTypeScene:
		if link.SceneID != nil {
			scene := rs.lookupScene(r, *link.SceneID)
			if scene != nil {
				e := manifestSceneEntry{
					ID:            scene.ID,
					Title:         scene.GetTitle(),
					Duration:      sceneDuration(scene),
					ScreenshotURL: fmt.Sprintf("/share/%s/scene/%d/screenshot", token, scene.ID),
					StreamURL:     fmt.Sprintf("/share/%s/scene/%d/stream", token, scene.ID),
					ViewerURL:     fmt.Sprintf("/share/%s/scene/%d/", token, scene.ID),
				}
				out.Scene = &e
			}
		}
	case models.ShareTypePerformer:
		if link.PerformerID != nil {
			performer, scenes, images := rs.lookupPerformerCatalog(r, *link.PerformerID)
			if performer != nil {
				p := buildPerformerEntry(performer, token)
				out.Performer = &p
				for _, sc := range scenes {
					out.Scenes = append(out.Scenes, manifestSceneEntry{
						ID:            sc.ID,
						Title:         sc.GetTitle(),
						Duration:      sceneDuration(sc),
						ScreenshotURL: fmt.Sprintf("/share/%s/scene/%d/screenshot", token, sc.ID),
						StreamURL:     fmt.Sprintf("/share/%s/scene/%d/stream", token, sc.ID),
						ViewerURL:     fmt.Sprintf("/share/%s/scene/%d/", token, sc.ID),
					})
				}
				for _, im := range images {
					out.Images = append(out.Images, manifestImageEntry{
						ID:           im.ID,
						Title:        im.GetTitle(),
						ImageURL:     fmt.Sprintf("/share/%s/image/%d/image", token, im.ID),
						ThumbnailURL: fmt.Sprintf("/share/%s/image/%d/thumbnail", token, im.ID),
						ViewerURL:    fmt.Sprintf("/share/%s/image/%d/", token, im.ID),
					})
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(out)
}

type manifestSceneEntry struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	Duration      float64 `json:"duration"`
	ScreenshotURL string  `json:"screenshot_url"`
	StreamURL     string  `json:"stream_url"`
	ViewerURL     string  `json:"viewer_url"`
}

type manifestImageEntry struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	ImageURL     string `json:"image_url"`
	ThumbnailURL string `json:"thumbnail_url"`
	ViewerURL    string `json:"viewer_url"`
}

type manifestPerformerEntry struct {
	Name      string `json:"name"`
	Aliases   string `json:"aliases"`
	Gender    string `json:"gender,omitempty"`
	Age       *int   `json:"age,omitempty"`
	Ethnicity string `json:"ethnicity,omitempty"`
	HairColor string `json:"hair_color,omitempty"`
	EyeColor  string `json:"eye_color,omitempty"`
	Height    *int   `json:"height_cm,omitempty"`
	FakeTits  string `json:"fake_tits,omitempty"`
	Career    string `json:"career_length,omitempty"`
	Country   string `json:"country,omitempty"`
	ImageURL  string `json:"image_url"`
}

type manifestResponse struct {
	ShareType string                  `json:"share_type"`
	Scene     *manifestSceneEntry     `json:"scene,omitempty"`
	Performer *manifestPerformerEntry `json:"performer,omitempty"`
	Scenes    []manifestSceneEntry    `json:"scenes,omitempty"`
	Images    []manifestImageEntry    `json:"images,omitempty"`
}

func (rs shareRoutes) lookupScene(r *http.Request, sceneID int) *models.Scene {
	var scene *models.Scene
	_ = rs.withReadTxn(r, func(ctx context.Context) error {
		s, err := rs.sceneFinder.Find(ctx, sceneID)
		if err == nil {
			scene = s
		}
		return nil
	})
	return scene
}

func (rs shareRoutes) lookupPerformerCatalog(r *http.Request, performerID int) (*models.Performer, []*models.Scene, []*models.Image) {
	var performer *models.Performer
	var scenes []*models.Scene
	var images []*models.Image
	_ = rs.withReadTxn(r, func(ctx context.Context) error {
		p, err := rs.performerFinder.Find(ctx, performerID)
		if err != nil || p == nil {
			return err
		}
		// Aliases is a lazy RelatedStrings — load before reading.
		if loader, ok := rs.performerFinder.(models.AliasLoader); ok {
			_ = p.LoadAliases(ctx, loader)
		}
		performer = p
		scenes, _ = rs.sceneFinder.(interface {
			FindByPerformerID(ctx context.Context, performerID int) ([]*models.Scene, error)
		}).FindByPerformerID(ctx, performerID)
		// Load primary file for each scene so duration is available on the tiles.
		for _, s := range scenes {
			if loadErr := s.LoadPrimaryFile(ctx, rs.fileGetter); loadErr != nil {
				logger.Warnf("[share] error loading primary file for scene %d: %v", s.ID, loadErr)
			}
		}
		images, _ = rs.imageFinder.(interface {
			FindByPerformerID(ctx context.Context, performerID int) ([]*models.Image, error)
		}).FindByPerformerID(ctx, performerID)
		return nil
	})
	return performer, scenes, images
}

// formatDuration renders a duration in seconds as MM:SS or H:MM:SS.
func formatDuration(secs float64) string {
	if secs <= 0 {
		return ""
	}
	total := int(secs + 0.5)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func sceneDuration(s *models.Scene) float64 {
	pf := s.Files.Primary()
	if pf == nil {
		return 0
	}
	return pf.Duration
}

func buildPerformerEntry(p *models.Performer, token string) manifestPerformerEntry {
	gender := ""
	if p.Gender != nil {
		gender = string(*p.Gender)
	}
	return manifestPerformerEntry{
		Name:      p.Name,
		Aliases:   strings.Join(p.Aliases.List(), ", "),
		Gender:    gender,
		Age:       computePerformerAge(p),
		Ethnicity: p.Ethnicity,
		HairColor: p.HairColor,
		EyeColor:  p.EyeColor,
		Height:    p.Height,
		FakeTits:  p.FakeTits,
		Career:    p.CareerLength,
		Country:   p.Country,
		ImageURL:  fmt.Sprintf("/share/%s/performer/image", token),
	}
}

func computePerformerAge(p *models.Performer) *int {
	if p.Birthdate == nil {
		return nil
	}
	now := time.Now()
	bd := p.Birthdate.Time
	age := now.Year() - bd.Year()
	if now.YearDay() < bd.YearDay() {
		age--
	}
	if age < 0 || age > 120 {
		return nil
	}
	return &age
}

// renderSceneViewerStandalone is the index for SCENE-type shares.
func (rs shareRoutes) renderSceneViewerStandalone(w http.ResponseWriter, r *http.Request, link *models.ShareLink) {
	if link.SceneID == nil {
		http.Error(w, "share has no scene", http.StatusInternalServerError)
		return
	}
	scene := rs.lookupSceneWithFile(r, *link.SceneID)
	if scene == nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	rs.renderSceneViewerForScene(w, r, link, scene)
}

func (rs shareRoutes) renderSceneViewerForScene(w http.ResponseWriter, r *http.Request, link *models.ShareLink, scene *models.Scene) {
	token := chi.URLParam(r, "token")
	title := scene.GetTitle()
	if title == "" {
		title = fmt.Sprintf("Scene #%d", scene.ID)
	}

	backHref := ""
	if link.ShareType == models.ShareTypePerformer {
		backHref = "/share/" + token + "/"
	}

	data := sceneViewerData{
		Title:     title,
		StreamURL: fmt.Sprintf("/share/%s/scene/%d/stream", token, scene.ID),
		PosterURL: fmt.Sprintf("/share/%s/scene/%d/screenshot", token, scene.ID),
		BackHref:  backHref,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", shareViewerCSP)
	w.Header().Set("X-Frame-Options", "DENY")
	if err := sceneViewerTemplate.Execute(w, data); err != nil {
		logger.Errorf("[share] error rendering scene viewer: %v", err)
	}
}

func (rs shareRoutes) lookupSceneWithFile(r *http.Request, sceneID int) *models.Scene {
	var scene *models.Scene
	_ = rs.withReadTxn(r, func(ctx context.Context) error {
		s, err := rs.sceneFinder.Find(ctx, sceneID)
		if err != nil {
			return err
		}
		if s != nil {
			if loadErr := s.LoadPrimaryFile(ctx, rs.fileGetter); loadErr != nil {
				logger.Warnf("[share] error loading primary file for scene %d: %v", s.ID, loadErr)
			}
		}
		scene = s
		return nil
	})
	return scene
}

func (rs shareRoutes) renderPerformerViewer(w http.ResponseWriter, r *http.Request, link *models.ShareLink) {
	if link.PerformerID == nil {
		http.Error(w, "share has no performer", http.StatusInternalServerError)
		return
	}
	performer, scenes, images := rs.lookupPerformerCatalog(r, *link.PerformerID)
	if performer == nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	token := chi.URLParam(r, "token")

	scenesOut := make([]sceneTile, 0, len(scenes))
	for _, s := range scenes {
		scenesOut = append(scenesOut, sceneTile{
			ID:            s.ID,
			Title:         s.GetTitle(),
			ScreenshotURL: fmt.Sprintf("/share/%s/scene/%d/screenshot", token, s.ID),
			PreviewURL:    fmt.Sprintf("/share/%s/scene/%d/webp", token, s.ID),
			ViewerURL:     fmt.Sprintf("/share/%s/scene/%d/", token, s.ID),
			Duration:      formatDuration(sceneDuration(s)),
		})
	}
	imagesOut := make([]imageTile, 0, len(images))
	for _, im := range images {
		imagesOut = append(imagesOut, imageTile{
			ID:           im.ID,
			Title:        im.GetTitle(),
			ThumbnailURL: fmt.Sprintf("/share/%s/image/%d/thumbnail", token, im.ID),
			ViewerURL:    fmt.Sprintf("/share/%s/image/%d/", token, im.ID),
		})
	}

	gender := ""
	if performer.Gender != nil {
		gender = string(*performer.Gender)
	}
	heightStr := ""
	if performer.Height != nil {
		// e.g. 165 cm (5′5″)
		feet := *performer.Height / 30
		inches := int(float64(*performer.Height)/2.54) % 12
		_ = feet
		_ = inches
		heightStr = fmt.Sprintf("%d cm", *performer.Height)
	}
	age := computePerformerAge(performer)
	ageStr := ""
	if age != nil {
		ageStr = strconv.Itoa(*age)
	}

	data := performerViewerData{
		Title:        performer.Name,
		Name:         performer.Name,
		Aliases:      strings.Join(performer.Aliases.List(), ", "),
		ImageURL:     fmt.Sprintf("/share/%s/performer/image", token),
		Gender:       gender,
		Age:          ageStr,
		Ethnicity:    performer.Ethnicity,
		HairColor:    performer.HairColor,
		EyeColor:     performer.EyeColor,
		Height:       heightStr,
		FakeTits:     performer.FakeTits,
		CareerLength: performer.CareerLength,
		Country:      performer.Country,
		Scenes:       scenesOut,
		Images:       imagesOut,
		HasScenes:    len(scenesOut) > 0,
		HasImages:    len(imagesOut) > 0,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", shareViewerCSP)
	w.Header().Set("X-Frame-Options", "DENY")
	if err := performerViewerTemplate.Execute(w, data); err != nil {
		logger.Errorf("[share] error rendering performer viewer: %v", err)
	}
}

type sceneViewerData struct {
	Title     string
	StreamURL string
	PosterURL string
	BackHref  string
}

type imageViewerData struct {
	Title    string
	ImageURL string
	BackHref string
}

type sceneTile struct {
	ID            int
	Title         string
	ScreenshotURL string
	PreviewURL    string
	ViewerURL     string
	Duration      string
}

type imageTile struct {
	ID           int
	Title        string
	ThumbnailURL string
	ViewerURL    string
}

type performerViewerData struct {
	Title        string
	Name         string
	Aliases      string
	ImageURL     string
	Gender       string
	Age          string
	Country      string
	Ethnicity    string
	HairColor    string
	EyeColor     string
	Height       string
	Measurements string
	FakeTits     string
	CareerLength string
	Scenes       []sceneTile
	Images       []imageTile
	HasScenes    bool
	HasImages    bool
}

const shareViewerCSP = "default-src 'none'; " +
	"img-src 'self'; " +
	"media-src 'self' blob:; " +
	"style-src 'unsafe-inline'; " +
	"font-src 'self'; " +
	"object-src 'none'; " +
	"frame-ancestors 'none'; " +
	"form-action 'none'; " +
	"base-uri 'none';"

var sceneViewerTemplate = template.Must(template.New("share-scene").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<meta name="referrer" content="no-referrer">
<meta name="robots" content="noindex, nofollow, noarchive">
<title>{{.Title}}</title>
<style>
:root { color-scheme: dark; }
* { box-sizing: border-box; }
html, body { margin: 0; padding: 0; height: 100%; background: #000; color: #eee; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
body { display: flex; flex-direction: column; min-height: 100vh; }
header { padding: 12px 16px; background: #111; border-bottom: 1px solid #222; display: flex; align-items: center; gap: 12px; }
header h1 { margin: 0; font-size: 1.05rem; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; min-width: 0; }
.back-link { color: #8af; text-decoration: none; font-size: 0.9rem; padding: 4px 8px; border: 1px solid #234; border-radius: 4px; }
.back-link:hover { background: #1a2230; }
main { flex: 1; display: flex; align-items: center; justify-content: center; background: #000; }
video { max-width: 100%; max-height: calc(100vh - 50px); width: 100%; background: #000; outline: none; }
footer { padding: 8px 16px; background: #111; border-top: 1px solid #222; font-size: 0.75rem; color: #888; text-align: center; }
</style>
</head>
<body>
<header>
{{if .BackHref}}<a class="back-link" href="{{.BackHref}}">← Back</a>{{end}}
<h1>{{.Title}}</h1>
</header>
<main>
<video controls playsinline preload="metadata" poster="{{.PosterURL}}" src="{{.StreamURL}}"></video>
</main>
<footer>Shared via Stash</footer>
</body>
</html>
`))

var imageViewerTemplate = template.Must(template.New("share-image").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<meta name="referrer" content="no-referrer">
<meta name="robots" content="noindex, nofollow, noarchive">
<title>{{.Title}}</title>
<style>
:root { color-scheme: dark; }
* { box-sizing: border-box; }
html, body { margin: 0; padding: 0; height: 100%; background: #000; color: #eee; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
body { display: flex; flex-direction: column; min-height: 100vh; }
header { padding: 12px 16px; background: #111; border-bottom: 1px solid #222; display: flex; align-items: center; gap: 12px; }
header h1 { margin: 0; font-size: 1.05rem; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; min-width: 0; }
.back-link { color: #8af; text-decoration: none; font-size: 0.9rem; padding: 4px 8px; border: 1px solid #234; border-radius: 4px; }
.back-link:hover { background: #1a2230; }
main { flex: 1; display: flex; align-items: center; justify-content: center; background: #000; padding: 8px; }
img.full { max-width: 100%; max-height: calc(100vh - 60px); object-fit: contain; }
footer { padding: 8px 16px; background: #111; border-top: 1px solid #222; font-size: 0.75rem; color: #888; text-align: center; }
</style>
</head>
<body>
<header>
{{if .BackHref}}<a class="back-link" href="{{.BackHref}}">← Back</a>{{end}}
<h1>{{.Title}}</h1>
</header>
<main>
<img class="full" src="{{.ImageURL}}" alt="">
</main>
<footer>Shared via Stash</footer>
</body>
</html>
`))

// performerViewerTemplate renders a profile + scene cards + image thumbnails
// using Stash's color palette and Blueprint dark theme conventions.
// Scenes show titles, durations, and play their animated webp preview on hover.
var performerViewerTemplate = template.Must(template.New("share-performer").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<meta name="referrer" content="no-referrer">
<meta name="robots" content="noindex, nofollow, noarchive">
<title>{{.Title}}</title>
<style>
:root { color-scheme: dark; }
* { box-sizing: border-box; }
html, body { margin: 0; padding: 0; background: #202b33; color: #f5f8fa; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif; -webkit-font-smoothing: antialiased; }
a { color: #48aff0; text-decoration: none; }

/* Profile header */
.profile {
  display: flex;
  gap: 28px;
  padding: 28px 24px;
  background: linear-gradient(180deg, #2c3a45 0%, #202b33 100%);
  border-bottom: 1px solid #394b59;
}
.profile .avatar-wrap {
  flex: 0 0 240px;
}
.profile .avatar {
  width: 240px;
  max-height: 360px;
  height: auto;
  object-fit: contain;
  border-radius: 6px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.5);
  display: block;
}
.profile .info { flex: 1; min-width: 0; }
.profile h1 { margin: 0 0 4px; font-size: 1.9rem; font-weight: 600; line-height: 1.1; }
.profile .alias { color: #bfccd6; font-size: 0.95rem; margin-bottom: 18px; }
.profile dl { display: grid; grid-template-columns: max-content 1fr; gap: 6px 18px; margin: 0; font-size: 0.92rem; max-width: 540px; }
.profile dt { color: #8a9ba8; font-weight: 500; }
.profile dd { margin: 0; color: #f5f8fa; }

@media (max-width: 720px) {
  .profile { flex-direction: column; align-items: center; padding: 20px 16px; gap: 16px; text-align: center; }
  .profile .avatar-wrap { flex: 0 0 auto; }
  .profile .avatar { width: 220px; max-width: 70vw; max-height: 50vh; }
  .profile dl { margin: 0 auto; max-width: 360px; }
}

/* Sections */
section { padding: 24px 16px; }
section h2 {
  font-size: 1.1rem;
  margin: 0 0 14px;
  color: #f5f8fa;
  font-weight: 600;
  letter-spacing: 0.02em;
  display: flex;
  align-items: baseline;
  gap: 10px;
}
section h2 .count { color: #8a9ba8; font-weight: 400; font-size: 0.9rem; }

/* Scene wall: cover + title overlay + animated preview on hover */
.wall {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 8px;
}
.tile {
  position: relative;
  display: block;
  aspect-ratio: 16/9;
  background: #30404d center/cover no-repeat;
  border-radius: 4px;
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(0,0,0,0.4);
  transition: transform 140ms ease, box-shadow 140ms ease;
}
.tile:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 14px rgba(0,0,0,0.6);
}
.tile .preview {
  position: absolute; inset: 0;
  background-size: cover; background-position: center;
  opacity: 0;
  transition: opacity 220ms ease;
}
.tile:hover .preview { opacity: 1; }
.tile .gradient {
  position: absolute; left: 0; right: 0; bottom: 0;
  height: 60%;
  background: linear-gradient(180deg, transparent 0%, rgba(0,0,0,0.85) 100%);
  pointer-events: none;
}
.tile .meta {
  position: absolute; left: 10px; right: 10px; bottom: 8px;
  color: #f5f8fa;
  pointer-events: none;
}
.tile .title {
  font-size: 0.9rem;
  font-weight: 500;
  line-height: 1.25;
  text-shadow: 0 1px 2px rgba(0,0,0,0.7);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.tile .duration {
  position: absolute; right: 8px; top: 8px;
  background: rgba(0,0,0,0.65);
  color: #f5f8fa;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 0.78rem;
  font-variant-numeric: tabular-nums;
  pointer-events: none;
}

/* Image grid */
.image-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 4px;
}
.image-grid a {
  display: block;
  aspect-ratio: 1/1;
  background: #30404d;
  border-radius: 3px;
  overflow: hidden;
  position: relative;
}
.image-grid img {
  width: 100%; height: 100%;
  object-fit: cover;
  display: block;
  transition: transform 200ms ease;
}
.image-grid a:hover img { transform: scale(1.05); }

footer { padding: 16px; text-align: center; color: #5c7080; font-size: 0.78rem; border-top: 1px solid #394b59; }
.empty { color: #8a9ba8; font-style: italic; }
</style>
</head>
<body>
<header class="profile">
  <div class="avatar-wrap"><img class="avatar" src="{{.ImageURL}}" alt=""></div>
  <div class="info">
    <h1>{{.Name}}</h1>
    {{if .Aliases}}<div class="alias">{{.Aliases}}</div>{{end}}
    <dl>
      {{if .Gender}}<dt>Gender</dt><dd>{{.Gender}}</dd>{{end}}
      {{if .Age}}<dt>Age</dt><dd>{{.Age}}</dd>{{end}}
      {{if .Ethnicity}}<dt>Ethnicity</dt><dd>{{.Ethnicity}}</dd>{{end}}
      {{if .HairColor}}<dt>Hair Color</dt><dd>{{.HairColor}}</dd>{{end}}
      {{if .EyeColor}}<dt>Eye Color</dt><dd>{{.EyeColor}}</dd>{{end}}
      {{if .Height}}<dt>Height</dt><dd>{{.Height}}</dd>{{end}}
      {{if .FakeTits}}<dt>Fake Tits</dt><dd>{{.FakeTits}}</dd>{{end}}
      {{if .CareerLength}}<dt>Career</dt><dd>{{.CareerLength}}</dd>{{end}}
      {{if .Country}}<dt>Country</dt><dd>{{.Country}}</dd>{{end}}
    </dl>
  </div>
</header>

<section>
  <h2>Scenes <span class="count">{{len .Scenes}}</span></h2>
  {{if .HasScenes}}
  <div class="wall">
    {{range .Scenes}}
    <a class="tile" href="{{.ViewerURL}}" title="{{.Title}}" style="background-image: url('{{.ScreenshotURL}}')">
      <div class="preview" style="background-image: url('{{.PreviewURL}}')"></div>
      {{if .Duration}}<div class="duration">{{.Duration}}</div>{{end}}
      <div class="gradient"></div>
      <div class="meta"><div class="title">{{.Title}}</div></div>
    </a>
    {{end}}
  </div>
  {{else}}<div class="empty">No scenes</div>{{end}}
</section>

<section>
  <h2>Images <span class="count">{{len .Images}}</span></h2>
  {{if .HasImages}}
  <div class="image-grid">
    {{range .Images}}<a href="{{.ViewerURL}}" title="{{.Title}}"><img src="{{.ThumbnailURL}}" alt="" loading="lazy"></a>{{end}}
  </div>
  {{else}}<div class="empty">No images</div>{{end}}
</section>

<footer>Shared via Stash</footer>
</body>
</html>
`))

func looksLikeShareToken(s string) bool {
	if len(s) < 32 || len(s) > 128 {
		return false
	}
	for _, c := range s {
		ok := (c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '_'
		if !ok {
			return false
		}
	}
	return true
}

func (rs shareRoutes) withTxnPlain(r *http.Request, fn func(ctx context.Context) error) error {
	return txn.WithTxn(r.Context(), rs.txnManager, fn)
}

// IsShareTokenPath returns true when the URL path is part of the public share subtree.
func IsShareTokenPath(path string) bool {
	return strings.HasPrefix(path, "/share/") || path == "/share"
}

var errShareLinkLimitReached = errors.New("share link view limit reached")
