package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"photo-backup/internal/backup"
	"photo-backup/internal/database"
	"photo-backup/internal/models"
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


// SelectDestFolder เปิด Dialog เลือกโฟลเดอร์ปลายทางสำหรับ Backup
func (a *App) SelectDestFolder() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("application is not ready")
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "เลือกโฟลเดอร์สำหรับเก็บรูปภาพ Backup",
	})
}
// StartBackup คัดลอกรูปจาก sourcePath ไป destPath
// แล้วบันทึกข้อมูลลง SQLite พร้อมส่ง Progress ผ่าน Wails Events
func (a *App) StartBackup(sourcePath string, destPath string) (backup.BackupSummary, error) {
	if a.repo == nil {
		return backup.BackupSummary{}, fmt.Errorf("database is not connected")
	}
	// 1. ค้นหารูปในโฟลเดอร์ต้นทาง
	images, err := backup.DiscoverImages(sourcePath)
	if err != nil {
		return backup.BackupSummary{}, err
	}
	if len(images) == 0 {
		return backup.BackupSummary{}, fmt.Errorf("ไม่พบรูปภาพในโฟลเดอร์ที่เลือก")
	}
	// 2. คัดลอกไฟล์ + ส่ง Progress event ไปให้ Frontend
	summary := backup.CopyFiles(images, destPath, func(current int, total int, name string) {
		// ส่ง event ให้ Frontend อัปเดต Progress bar
		runtime.EventsEmit(a.ctx, "backup:progress", map[string]interface{}{
			"current": current,
			"total":   total,
			"name":    name,
		})
	})
	// 3. บันทึกรูปที่ copy สำเร็จลง SQLite
	for _, result := range summary.Results {
		if !result.Success {
			continue
		}
		photo := &models.Photo{
			OriginalName: result.OriginalName,
			OriginalPath: result.OriginalPath,
			BackupPath:   result.BackupPath,
			FileSize:     result.FileSize,
			BackedUpAt:   time.Now(),
		}
		if dbErr := a.repo.CreatePhoto(photo); dbErr != nil {
			// ถ้าบันทึก DB ไม่ได้ ไม่หยุดทั้งหมด แค่ log ไว้
			fmt.Printf("Warning: save photo to db failed: %s\n", dbErr.Error())
		}
	}
	return summary, nil
}
// GetBackupFolder คืนค่าตำแหน่ง Backup เริ่มต้น (สำหรับแนะนำ path ให้ผู้ใช้)
func (a *App) GetBackupFolder() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "PhotoBackup")
}
