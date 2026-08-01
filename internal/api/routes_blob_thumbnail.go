package api

import (
	"errors"
	"net/http"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/utils"
)

// Maximum dimension of the generated thumbnails for blob-stored images, chosen
// as 2x the CSS size of the card each one is displayed in so they stay sharp on
// HiDPI screens:
//
//	performer cards render at 248x372 -> 744 on the long edge
//	scene cards render at 382x220     -> 764 on the long edge
//
// Originals are typically 1280x1920 and 1920x1080 respectively, so the grids
// were downloading roughly an order of magnitude more pixels than they drew.
const (
	performerThumbnailSize  = 750
	sceneCoverThumbnailSize = 768
)

// serveBlobThumbnail serves a downscaled JPEG of a blob-stored image, generating
// it on first request and caching it on disk. It reports whether it served the
// response; on false the caller should fall back to serving the original.
//
// checksum is the blob checksum, which addresses the cache: it changes exactly
// when the image does, so unlike an updated_at key it neither goes stale after
// an image swap nor orphans a file every time some unrelated field is edited.
// Passing nil (no image stored) declines, leaving default images alone.
//
// readBlob is only called on a cache miss, so a warm thumbnail costs one indexed
// checksum lookup and a static file serve — the original blob is never read.
func serveBlobThumbnail(w http.ResponseWriter, r *http.Request, checksum *string, maxSize int, readBlob func() ([]byte, error)) bool {
	if checksum == nil || *checksum == "" {
		return false
	}

	mgr := manager.GetInstance()
	filepath := mgr.Paths.Generated.GetBlobThumbnailPath(*checksum, maxSize)

	if exists, _ := fsutil.FileExists(filepath); exists {
		utils.ServeStaticFile(w, r, filepath)
		return true
	}

	// Bound concurrent encodes the same way the image thumbnail route does; a
	// cold grid otherwise fires ~40 vips processes at once.
	wg := &mgr.ImageThumbnailGenerateWaitGroup
	wg.Add()
	defer wg.Done()

	// Another request may have generated it while we waited for the gate.
	if exists, _ := fsutil.FileExists(filepath); exists {
		utils.ServeStaticFile(w, r, filepath)
		return true
	}

	data, err := readBlob()
	if err != nil || len(data) == 0 {
		return false
	}

	encoder := image.NewThumbnailEncoder(mgr.FFMpeg, mgr.FFProbe, image.ClipPreviewOptions{
		InputArgs:  mgr.Config.GetTranscodeInputArgs(),
		OutputArgs: mgr.Config.GetTranscodeOutputArgs(),
		Preset:     mgr.Config.GetPreviewPreset().String(),
	})

	thumb, err := encoder.GetThumbnailFromData(data, maxSize)
	if err != nil {
		// Animated images are declined by design, so don't log those.
		if !errors.Is(err, image.ErrNotSupportedForThumbnail) {
			logger.Errorf("error generating thumbnail for blob %s: %v", *checksum, err)
		}
		return false
	}

	// A tiny original can encode larger than itself once re-compressed. Serving
	// the original is both smaller and sharper, so don't cache a worse file.
	if len(thumb) >= len(data) {
		return false
	}

	if err := fsutil.WriteFile(filepath, thumb); err != nil {
		logger.Errorf("error writing thumbnail for blob %s: %v", *checksum, err)
		utils.ServeStaticContent(w, r, thumb)
		return true
	}

	utils.ServeStaticFile(w, r, filepath)
	return true
}
