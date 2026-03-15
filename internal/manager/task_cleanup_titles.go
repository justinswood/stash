package manager

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/scene"
)

type cleanupTitlesJob struct {
	repository models.Repository
}

var cleanupTitleHashSuffix = regexp.MustCompile(`#(\d+)$`)

func (j *cleanupTitlesJob) Execute(ctx context.Context, progress *job.Progress) error {
	sceneCount, err := j.getSceneCount(ctx)
	if err != nil {
		return err
	}

	imageCount, err := j.getImageCount(ctx)
	if err != nil {
		return err
	}

	total := sceneCount + imageCount
	progress.SetTotal(total)
	logger.Infof("Starting title cleanup for %d scenes and %d images", sceneCount, imageCount)

	updatedScenes := j.processScenes(ctx, progress)
	updatedImages := j.processImages(ctx, progress)

	logger.Infof("Finished title cleanup. Updated %d scenes and %d images", updatedScenes, updatedImages)
	return nil
}

func (j *cleanupTitlesJob) processScenes(ctx context.Context, progress *job.Progress) int {
	const batchSize = 1000
	findFilter := models.BatchFindFilter(batchSize)
	sceneFilter := &models.SceneFilterType{}

	updated := 0
	more := true
	for more {
		if job.IsCancelled(ctx) {
			logger.Info("Stopping title cleanup due to user request")
			return updated
		}

		var scenes []*models.Scene
		if err := j.repository.WithReadTxn(ctx, func(ctx context.Context) error {
			var err error
			scenes, err = scene.Query(ctx, j.repository.Scene, sceneFilter, findFilter)
			return err
		}); err != nil {
			if !job.IsCancelled(ctx) {
				logger.Errorf("error querying scenes for title cleanup: %v", err)
			}
			return updated
		}

		for _, s := range scenes {
			if job.IsCancelled(ctx) {
				logger.Info("Stopping title cleanup due to user request")
				return updated
			}

			updatedScene, err := j.cleanupSceneTitle(ctx, s)
			if err != nil {
				logger.Errorf("error cleaning up title for scene %d: %v", s.ID, err)
			} else if updatedScene {
				updated++
			}

			progress.Increment()
		}

		if len(scenes) != batchSize {
			more = false
		} else {
			*findFilter.Page++

			if *findFilter.Page%10 == 1 {
				logger.Infof("Processed %d scenes...", (*findFilter.Page-1)*batchSize)
			}
		}
	}

	return updated
}

func (j *cleanupTitlesJob) processImages(ctx context.Context, progress *job.Progress) int {
	const batchSize = 1000
	findFilter := models.BatchFindFilter(batchSize)
	imageFilter := &models.ImageFilterType{}

	updated := 0
	more := true
	for more {
		if job.IsCancelled(ctx) {
			logger.Info("Stopping title cleanup due to user request")
			return updated
		}

		var images []*models.Image
		if err := j.repository.WithReadTxn(ctx, func(ctx context.Context) error {
			var err error
			images, err = image.Query(ctx, j.repository.Image, imageFilter, findFilter)
			return err
		}); err != nil {
			if !job.IsCancelled(ctx) {
				logger.Errorf("error querying images for title cleanup: %v", err)
			}
			return updated
		}

		for _, img := range images {
			if job.IsCancelled(ctx) {
				logger.Info("Stopping title cleanup due to user request")
				return updated
			}

			updatedImage, err := j.cleanupImageTitle(ctx, img)
			if err != nil {
				logger.Errorf("error cleaning up title for image %d: %v", img.ID, err)
			} else if updatedImage {
				updated++
			}

			progress.Increment()
		}

		if len(images) != batchSize {
			more = false
		} else {
			*findFilter.Page++

			if *findFilter.Page%10 == 1 {
				logger.Infof("Processed %d images...", (*findFilter.Page-1)*batchSize)
			}
		}
	}

	return updated
}

func (j *cleanupTitlesJob) getSceneCount(ctx context.Context) (int, error) {
	r := j.repository
	var count int
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		count, err = r.Scene.QueryCount(ctx, &models.SceneFilterType{}, nil)
		return err
	}); err != nil {
		return 0, err
	}

	return count, nil
}

func (j *cleanupTitlesJob) getImageCount(ctx context.Context) (int, error) {
	r := j.repository
	var count int
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		count, err = r.Image.QueryCount(ctx, &models.ImageFilterType{}, nil)
		return err
	}); err != nil {
		return 0, err
	}

	return count, nil
}

func (j *cleanupTitlesJob) cleanupImageTitle(ctx context.Context, img *models.Image) (bool, error) {
	if img.Path == "" {
		return false, nil
	}

	filename := filepath.Base(img.Path)
	filenameNoExt := strings.TrimSuffix(filename, filepath.Ext(filename))

	performer, title, ok := parseCleanupTitle(filename)
	if !ok {
		return false, nil
	}

	if !shouldUpdateTitle(img.Title, filename, filenameNoExt, performer, title) {
		return false, nil
	}

	if img.Title == title {
		return false, nil
	}

	if err := j.repository.WithTxn(ctx, func(ctx context.Context) error {
		partial := models.NewImagePartial()
		partial.Title = models.NewOptionalString(title)
		_, err := j.repository.Image.UpdatePartial(ctx, img.ID, partial)
		return err
	}); err != nil {
		return false, err
	}

	return true, nil
}

func (j *cleanupTitlesJob) cleanupSceneTitle(ctx context.Context, s *models.Scene) (bool, error) {
	if s.Path == "" {
		return false, nil
	}

	filename := filepath.Base(s.Path)
	filenameNoExt := strings.TrimSuffix(filename, filepath.Ext(filename))

	performer, title, ok := parseCleanupTitle(filename)
	if !ok {
		return false, nil
	}

	if !shouldUpdateTitle(s.Title, filename, filenameNoExt, performer, title) {
		return false, nil
	}

	if s.Title == title {
		return false, nil
	}

	if err := j.repository.WithTxn(ctx, func(ctx context.Context) error {
		partial := models.NewScenePartial()
		partial.Title = models.NewOptionalString(title)
		_, err := j.repository.Scene.UpdatePartial(ctx, s.ID, partial)
		return err
	}); err != nil {
		return false, err
	}

	return true, nil
}

func parseCleanupTitle(filename string) (string, string, bool) {
	if !strings.Contains(filename, " - ") {
		return "", "", false
	}

	parts := strings.SplitN(filename, " - ", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	performer := normalizeWhitespace(parts[0])
	if performer == "" {
		return "", "", false
	}

	rest := parts[1]
	ext := filepath.Ext(rest)
	if ext == "" {
		return "", "", false
	}

	title := strings.TrimSuffix(rest, ext)
	title = cleanupTitleHashSuffix.ReplaceAllString(title, "$1")
	title = normalizeWhitespace(title)
	if title == "" {
		return "", "", false
	}

	return performer, title, true
}

func shouldUpdateTitle(currentTitle, filename, filenameNoExt, performer, newTitle string) bool {
	if currentTitle == "" || currentTitle == filename || currentTitle == filenameNoExt {
		return true
	}

	if strings.HasPrefix(currentTitle, performer+" - ") &&
		strings.HasSuffix(currentTitle, newTitle) {
		return true
	}

	return false
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
