package repository

import (
	"errors"
	"path/filepath"
	"strings"

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

func (r *Repository) PhotosInDestination(destPath string) ([]model.Photo, error) {
	var photos []model.Photo
	if err := r.db.Select("id, original_name, backup_path, file_size, content_hash, backed_up_at").Find(&photos).Error; err != nil {
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

// บันทึกรูป ประวัติ และแท็กสีให้สำเร็จพร้อมกันใน transaction เดียว
func (r *Repository) SaveBackup(photo *model.Photo) error {
	photo.ColorTag = strings.TrimSpace(photo.ColorTag)
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
			previousColor := previous.ColorTag
			colorTag := photo.ColorTag
			if colorTag == "" {
				colorTag = previousColor
			}
			updates := map[string]interface{}{
				"file_size": photo.FileSize, "source_modified_at": photo.SourceModifiedAt,
				"backed_up_at": photo.BackedUpAt, "color_tag": colorTag,
			}
			contentChanged := photo.ContentHash != nil && previous.ContentHash != nil && *photo.ContentHash != *previous.ContentHash
			if strings.TrimSpace(photo.Description) != "" || contentChanged {
				updates["description"] = photo.Description
			}
			if photo.ContentHash != nil && strings.TrimSpace(*photo.ContentHash) != "" {
				updates["content_hash"] = photo.ContentHash
			}
			// Update the table model without passing the preloaded Tags slice to
			// GORM association callbacks; provenance is managed explicitly below.
			if err := tx.Model(&model.Photo{}).Where("id = ?", previous.ID).Updates(updates).Error; err != nil {
				return err
			}
			if contentChanged {
				if err := tx.Where("photo_id = ? AND source = ?", previous.ID, "ai").Delete(&model.PhotoTag{}).Error; err != nil {
					return err
				}
			}
			if previousColor != "" && previousColor != colorTag {
				for _, tag := range previous.Tags {
					if tag.Name == previousColor {
						if err := tx.Where("photo_id = ? AND tag_id = ? AND source = ?", previous.ID, tag.ID, "color").Delete(&model.PhotoTag{}).Error; err != nil {
							return err
						}
					}
				}
			}
		}
		if photo.ColorTag != "" {
			var tag model.Tag
			if err := tx.Where("name = ?", photo.ColorTag).
				FirstOrCreate(&tag, model.Tag{Name: photo.ColorTag}).Error; err != nil {
				return err
			}
			if err := appendPhotoTag(tx, photo.ID, tag.ID, "color"); err != nil {
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

// SaveAIAnalysis stores the AI description and associates the generated tags
// with a photo in one transaction.
func (r *Repository) SaveAIAnalysis(photoID uint, description string, tagNames []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var photo model.Photo
		if err := tx.First(&photo, photoID).Error; err != nil {
			return err
		}
		if err := tx.Model(&photo).Update("description", strings.TrimSpace(description)).Error; err != nil {
			return err
		}
		if err := tx.Where("photo_id = ? AND source = ?", photo.ID, "ai").Delete(&model.PhotoTag{}).Error; err != nil {
			return err
		}
		seen := make(map[string]bool)
		for _, name := range tagNames {
			name = strings.TrimSpace(name)
			key := strings.ToLower(name)
			if name == "" || seen[key] {
				continue
			}
			seen[key] = true
			var tag model.Tag
			if err := tx.Where("name = ?", name).FirstOrCreate(&tag, model.Tag{Name: name}).Error; err != nil {
				return err
			}
			if err := appendPhotoTag(tx, photo.ID, tag.ID, "ai"); err != nil {
				return err
			}
		}
		return nil
	})
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
