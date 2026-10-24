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
			if err := tx.Model(&previous).Updates(map[string]interface{}{
				"file_size": photo.FileSize, "source_modified_at": photo.SourceModifiedAt,
				"backed_up_at": photo.BackedUpAt, "color_tag": photo.ColorTag,
			}).Error; err != nil {
				return err
			}
			if previousColor != "" && previousColor != photo.ColorTag {
				for _, tag := range previous.Tags {
					if tag.Name == previousColor {
						if err := tx.Model(&previous).Association("Tags").Delete(&tag); err != nil {
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
			if err := tx.Model(photo).Association("Tags").Append(&tag); err != nil {
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
