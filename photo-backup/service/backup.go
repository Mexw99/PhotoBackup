package service

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"photo-backup/backup"
	"photo-backup/model"
)

func ListImageFiles(sourcePath string) ([]backup.ImageFile, error) {
	return backup.DiscoverImages(sourcePath)
}

func DefaultBackupFolder() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "PhotoBackup")
}

// StartBackup keeps the existing transfer behavior and saves successful results.
// The callback lets the presentation layer publish progress through Wails.
func (s *Service) StartBackup(sourcePath, destPath string, onProgress backup.ProgressCallback) (backup.BackupSummary, error) {
	images, err := backup.DiscoverImages(sourcePath)
	if err != nil {
		return backup.BackupSummary{}, err
	}
	if len(images) == 0 {
		return backup.BackupSummary{}, fmt.Errorf("ไม่พบรูปภาพในโฟลเดอร์ที่เลือก")
	}

	summary := backup.CopyFiles(images, destPath, onProgress)
	for _, result := range summary.Results {
		if !result.Success {
			continue
		}
		photo := &model.Photo{
			OriginalName: result.OriginalName,
			OriginalPath: result.OriginalPath,
			BackupPath:   result.BackupPath,
			FileSize:     result.FileSize,
			BackedUpAt:   time.Now(),
		}
		// Give each write its own timeout; a large transfer may take longer.
		store, cancel := s.operation()
		dbErr := store.CreatePhoto(photo)
		cancel()
		if dbErr != nil {
			fmt.Printf("Warning: save photo to db failed: %s\n", dbErr.Error())
		}
	}
	return summary, nil
}
