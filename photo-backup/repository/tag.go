package repository

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"photo-backup/model"
)

func (r *Repository) CreateTag(name string) (*model.Tag, error) {
	name = normalizeTagName(name)

	if name == "" {
		return nil, ErrEmptyTagName
	}

	return findOrCreateTag(r.db, name)
}

func normalizeTagName(name string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(name)), " ")
}

func findOrCreateTag(db *gorm.DB, name string) (*model.Tag, error) {
	var tag model.Tag
	err := db.Where("LOWER(name) = LOWER(?)", name).First(&tag).Error
	if err == nil {
		return &tag, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	tag.Name = name
	return &tag, db.Create(&tag).Error
}

func appendPhotoTag(db *gorm.DB, photoID, tagID uint, source string) error {
	if source == "" {
		source = "user"
	}
	// Use the explicit three-column key. GORM's many-to-many helper knows
	// about the legacy two-column join table and can otherwise default source
	// back to "user" when inserting an AI or color relationship.
	return db.Exec("INSERT OR IGNORE INTO photo_tags (photo_id, tag_id, source) VALUES (?, ?, ?)", photoID, tagID, source).Error
}

func (r *Repository) ListTags() ([]model.Tag, error) {
	var tags []model.Tag

	err := r.db.
		Order("name ASC").
		Find(&tags).
		Error

	return tags, err
}

func (r *Repository) CountTags() (int64, error) {
	var count int64

	err := r.db.
		Model(&model.Tag{}).
		Count(&count).
		Error

	return count, err
}

var (
	ErrPhotoNotFound = errors.New("photo not found")
	ErrTagNotFound   = errors.New("tag not found")
	ErrNoPhotoIDs    = errors.New("select at least one photo")
)

type TagCount struct {
	ID         uint
	Name       string
	PhotoCount int64
}

// เพิ่ม Tag เดียวให้รูปหลายรูป และบันทึกความสัมพันธ์ใน photo_tags
func (r *Repository) AddTagToPhotos(photoIDs []uint, name string) (*model.Tag, error) {
	name = normalizeTagName(name)
	if name == "" {
		return nil, ErrEmptyTagName
	}
	if len(photoIDs) == 0 {
		return nil, ErrNoPhotoIDs
	}

	// ตัด ID ซ้ำก่อนนำไปเปรียบเทียบจำนวนรูปที่พบ
	uniqueIDs := make([]uint, 0, len(photoIDs))
	seen := make(map[uint]bool)
	for _, id := range photoIDs {
		if !seen[id] {
			seen[id] = true
			uniqueIDs = append(uniqueIDs, id)
		}
	}

	var tag model.Tag

	err := r.db.Transaction(func(tx *gorm.DB) error {
		// ใช้ Tag เดิมเมื่อชื่อเหมือนกันโดยไม่สนใจตัวพิมพ์และช่องว่างซ้ำ
		createdTag, err := findOrCreateTag(tx, name)
		if err != nil {
			return err
		}
		tag = *createdTag

		var photos []model.Photo
		if err := tx.Where("id IN ?", uniqueIDs).Find(&photos).Error; err != nil {
			return err
		}
		if len(photos) != len(uniqueIDs) {
			return ErrPhotoNotFound
		}

		for _, photo := range photos {
			if err := appendPhotoTag(tx, photo.ID, tag.ID, "user"); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &tag, nil
}

// ลบเฉพาะความสัมพันธ์ระหว่างรูปกับ Tag ไม่ลบ Tag จากฐานข้อมูล
func (r *Repository) RemoveTagFromPhoto(photoID, tagID uint) error {
	var photo model.Photo
	if err := r.db.First(&photo, photoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPhotoNotFound
		}
		return err
	}

	var tag model.Tag
	if err := r.db.First(&tag, tagID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTagNotFound
		}
		return err
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("photo_id = ? AND tag_id = ? AND source IN ?", photo.ID, tag.ID, []string{"user", "color"}).Delete(&model.PhotoTag{}).Error; err != nil {
			return err
		}
		if photo.ColorTag == tag.Name {
			return tx.Model(&photo).Update("color_tag", "").Error
		}
		return nil
	})
}

// DeleteTag removes the tag and all photo associations in one transaction.
func (r *Repository) DeleteTag(tagID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var tag model.Tag
		if err := tx.First(&tag, "id = ?", tagID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTagNotFound
			}
			return err
		}

		if err := tx.Where("tag_id = ?", tag.ID).Delete(&model.PhotoTag{}).Error; err != nil {
			return err
		}
		// ล้างชื่อสีที่เก็บไว้ด้วย ไม่ให้ UI แสดงแท็กที่ถูกลบไปแล้ว
		if err := tx.Model(&model.Photo{}).Where("color_tag = ?", tag.Name).
			Update("color_tag", "").Error; err != nil {
			return err
		}
		return tx.Delete(&tag).Error
	})
}

// คืน Tag พร้อมจำนวนรูป เรียงจำนวนมากไปน้อย
func (r *Repository) GetTagCloud() ([]TagCount, error) {
	var result []TagCount

	err := r.db.Model(&model.Tag{}).
		Select("tags.id, tags.name, COUNT(DISTINCT photo_tags.photo_id) AS photo_count").
		Joins("LEFT JOIN photo_tags ON photo_tags.tag_id = tags.id").
		Group("tags.id, tags.name").
		Order("photo_count DESC").
		Order("tags.name ASC").
		Scan(&result).Error

	return result, err
}
