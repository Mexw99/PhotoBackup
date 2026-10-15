package main

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"photo-backup/internal/backup"
	"photo-backup/internal/database"
	"photo-backup/internal/repository"
)

type App struct {
	ctx      context.Context
	database *database.Database
	repo     *repository.Repository
	dbPath   string
	dbErr    error
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	db, path, err := database.OpenDefault()
	if err != nil {
		a.dbErr = err
		return
	}

	a.database = db
	a.repo = repository.New(db.GORM())
	a.dbPath = path
}

func (a *App) shutdown(_ context.Context) {
	if a.database != nil {
		_ = a.database.Close()
	}
}

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// SelectSourceFolder opens the native folder picker for the image source.
func (a *App) SelectSourceFolder() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("application is not ready")
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "เลือกโฟลเดอร์รูปภาพต้นทาง",
	})
}

// ListImageFiles previews supported images before the copy operation is started.
func (a *App) ListImageFiles(sourcePath string) ([]backup.ImageFile, error) {
	return backup.DiscoverImages(sourcePath)
}

func (a *App) GetDatabaseStatus() string {
	if a.dbErr != nil || a.database == nil || a.repo == nil {
		return "Disconnected"
	}

	return "Connected"
}

func (a *App) GetDatabasePath() string {
	return a.dbPath
}

func (a *App) GetPhotoCount() int64 {
	if a.repo == nil {
		return 0
	}

	count, err := a.repo.CountPhotos()
	if err != nil {
		return 0
	}

	return count
}

func (a *App) GetTagCount() int64 {
	if a.repo == nil {
		return 0
	}

	count, err := a.repo.CountTags()
	if err != nil {
		return 0
	}

	return count
}

type TagSummary struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

func (a *App) GetTags() []TagSummary {
	if a.repo == nil {
		return []TagSummary{}
	}

	tags, err := a.repo.ListTags()
	if err != nil {
		return []TagSummary{}
	}

	result := make([]TagSummary, 0, len(tags))

	for _, tag := range tags {
		result = append(result, TagSummary{
			ID:   tag.ID,
			Name: tag.Name,
		})
	}

	return result
}

type PhotoSummary struct {
	ID           uint   `json:"id"`
	OriginalName string `json:"originalName"`
	BackupPath   string `json:"backupPath"`
	BackedUpAt   string `json:"backedUpAt"`
}

func (a *App) GetRecentPhotos(limit int) []PhotoSummary {
	if a.repo == nil {
		return []PhotoSummary{}
	}

	photos, err := a.repo.RecentPhotos(limit)
	if err != nil {
		return []PhotoSummary{}
	}

	result := make([]PhotoSummary, 0, len(photos))

	for _, photo := range photos {
		result = append(result, PhotoSummary{
			ID:           photo.ID,
			OriginalName: photo.OriginalName,
			BackupPath:   photo.BackupPath,
			BackedUpAt:   photo.BackedUpAt.Format("2006-01-02 15:04:05"),
		})
	}

	return result
}
