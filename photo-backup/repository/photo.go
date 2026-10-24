package repository

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"photo-backup/model"
)

var ErrBackupPathConflict = errors.New("backup path belongs to another source")

func (r *Repository) CreatePhoto(photo *model.Photo) error {
	return r.db.Create(photo).Error
}

// FindBackup ดูประวัติล่าสุดของรูปที่สำรองไว้ในปลายทางที่กำลังเลือก
func (r *Repository) FindBackup(sourcePath, destPath string) (*model.Photo, error) {
	var photos []model.Photo
	err := r.db.Where("original_path = ?", sourcePath).
		Order("backed_up_at DESC").Order("id DESC").Find(&photos).Error
	if err != nil {
		return nil, err
	}
	for _, photo := range photos {
		relative, err := filepath.Rel(destPath, photo.BackupPath)
		if err != nil {
			continue
		}
		// Rel ช่วยแยก backup-A ออกจาก backup-A-other โดยไม่เทียบแค่ prefix
		if relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return &photo, nil
		}
	}
	return nil, nil
}

// BackupPathsInDestination loads only backup paths so the service can detect
// duplicate destinations once before starting the parallel workers.
func (r *Repository) BackupPathsInDestination(destPath string) ([]string, error) {
	var photos []model.Photo
	if err := r.db.Select("backup_path").Find(&photos).Error; err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(photos))
	for _, photo := range photos {
		relative, err := filepath.Rel(destPath, photo.BackupPath)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		paths = append(paths, filepath.Clean(photo.BackupPath))
	}
	return paths, nil
}

// PhotosInDestination returns only the metadata needed for an integrity scan.
// Tags are intentionally not preloaded because this operation checks files.
func (r *Repository) PhotosInDestination(destPath string) ([]model.Photo, error) {
	var photos []model.Photo
	if err := r.db.Select("id, original_name, backup_path, file_size, backed_up_at").Find(&photos).Error; err != nil {
		return nil, err
	}

	selected := make([]model.Photo, 0, len(photos))
	for _, photo := range photos {
		relative, err := filepath.Rel(destPath, photo.BackupPath)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		selected = append(selected, photo)
	}
	return selected, nil
}

// SaveBackup บันทึกรูปและประวัติใน transaction เดียว
func (r *Repository) SaveBackup(photo *model.Photo) error {
	photo.AnalysisStatus = model.AnalysisPending
	photo.AnalysisError = ""
	photo.ContentHash = ""
	photo.AnalyzedAt = nil
	return r.db.Transaction(func(tx *gorm.DB) error {
		var previous model.Photo
		err := tx.Preload("Tags").Where("backup_path = ?", photo.BackupPath).First(&previous).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(photo).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			if previous.OriginalPath != photo.OriginalPath {
				return ErrBackupPathConflict
			}
			// เมื่อไฟล์สำเนาหาย ใช้แถวเดิมและคงแท็กที่ผู้ใช้เพิ่มไว้
			photo.ID = previous.ID
			if err := tx.Model(&previous).Updates(map[string]interface{}{
				"file_size": photo.FileSize, "source_modified_at": photo.SourceModifiedAt,
				"backed_up_at": photo.BackedUpAt, "description": "",
				"analysis_status": model.AnalysisPending, "analysis_error": "",
				"content_hash": "", "analyzed_at": nil,
			}).Error; err != nil {
				return err
			}
		}
		history := model.FileRecord{
			Dest: filepath.Dir(photo.BackupPath), Filename: filepath.Base(photo.BackupPath),
			Status: "backed_up",
		}
		return tx.Create(&history).Error
	})
}

func (r *Repository) GetPhoto(id uint) (*model.Photo, error) {
	var photo model.Photo
	if err := r.db.Preload("Tags").First(&photo, id).Error; err != nil {
		return nil, err
	}
	return &photo, nil
}

func (r *Repository) PendingAnalysisPhotoIDs() ([]uint, error) {
	var ids []uint
	if err := r.db.Model(&model.Photo{}).
		Where("analysis_status IN ?", []string{model.AnalysisPending, model.AnalysisProcessing}).
		Order("id ASC").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *Repository) FailedAnalysisPhotoIDs() ([]uint, error) {
	var ids []uint
	if err := r.db.Model(&model.Photo{}).
		Where("analysis_status = ?", model.AnalysisFailed).
		Order("id ASC").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *Repository) SetPhotoAnalysisStatus(id uint, status, message string) error {
	return r.db.Model(&model.Photo{}).Where("id = ?", id).Updates(map[string]interface{}{
		"analysis_status": status,
		"analysis_error":  message,
	}).Error
}

func (r *Repository) SavePhotoAnalysis(id uint, description string, tagNames []string, contentHash string, analyzedAt time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var photo model.Photo
		if err := tx.First(&photo, id).Error; err != nil {
			return err
		}
		var previousAI []model.PhotoTag
		if err := tx.Where("photo_id = ? AND source = ?", id, model.TagSourceAI).Find(&previousAI).Error; err != nil {
			return err
		}
		for _, link := range previousAI {
			if err := tx.Delete(&link).Error; err != nil {
				return err
			}
			var relationCount int64
			if err := tx.Model(&model.PhotoTag{}).Where("tag_id = ?", link.TagID).Count(&relationCount).Error; err != nil {
				return err
			}
			if relationCount == 0 {
				if err := tx.Delete(&model.Tag{}, link.TagID).Error; err != nil {
					return err
				}
			}
		}
		for _, name := range tagNames {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			var tag model.Tag
			if err := tx.Where("name = ?", name).
				FirstOrCreate(&tag, model.Tag{Name: name}).Error; err != nil {
				return err
			}
			var existing model.PhotoTag
			err := tx.Where("photo_id = ? AND tag_id = ?", photo.ID, tag.ID).First(&existing).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(&model.PhotoTag{PhotoID: photo.ID, TagID: tag.ID, Source: model.TagSourceAI}).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
		}
		return tx.Model(&photo).Updates(map[string]interface{}{
			"description":     strings.TrimSpace(description),
			"analysis_status": model.AnalysisCompleted,
			"analysis_error":  "",
			"content_hash":    contentHash,
			"analyzed_at":     analyzedAt,
		}).Error
	})
}

type AnalysisCounts struct {
	Pending    int64
	Processing int64
	Completed  int64
	Failed     int64
}

func (r *Repository) AnalysisCounts() (AnalysisCounts, error) {
	counts := AnalysisCounts{}
	for status, target := range map[string]*int64{
		model.AnalysisPending:    &counts.Pending,
		model.AnalysisProcessing: &counts.Processing,
		model.AnalysisCompleted:  &counts.Completed,
		model.AnalysisFailed:     &counts.Failed,
	} {
		if err := r.db.Model(&model.Photo{}).Where("analysis_status = ?", status).Count(target).Error; err != nil {
			return AnalysisCounts{}, err
		}
	}
	return counts, nil
}

func (r *Repository) ListPhotos() ([]model.Photo, error) {
	var photos []model.Photo

	err := r.db.
		Preload("Tags").
		Order("backed_up_at DESC").
		Find(&photos).
		Error

	return photos, err
}

func (r *Repository) RecentPhotos(limit int) ([]model.Photo, error) {
	if limit <= 0 {
		limit = 5
	}

	var photos []model.Photo

	err := r.db.
		Preload("Tags").
		Order("backed_up_at DESC").
		Limit(limit).
		Find(&photos).
		Error

	return photos, err
}

func (r *Repository) CountPhotos() (int64, error) {
	var count int64

	err := r.db.
		Model(&model.Photo{}).
		Count(&count).
		Error

	return count, err
}
