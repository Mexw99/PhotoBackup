package repository

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"photo-backup/model"
)

func (r *Repository) CreateTag(name string) (*model.Tag, error) {
	name = strings.TrimSpace(name)

	if name == "" {
		return nil, ErrEmptyTagName
	}

	tag := &model.Tag{
		Name: name,
	}

	err := r.db.
		Where("name = ?", name).
		FirstOrCreate(tag).
		Error

	return tag, err
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
	name = strings.TrimSpace(name)
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
		// FirstOrCreate ป้องกันการสร้าง Tag ชื่อซ้ำ
		if err := tx.Where("name = ?", name).
			FirstOrCreate(&tag, model.Tag{Name: name}).Error; err != nil {
			return err
		}

		var photos []model.Photo
		if err := tx.Where("id IN ?", uniqueIDs).Find(&photos).Error; err != nil {
			return err
		}
		if len(photos) != len(uniqueIDs) {
			return ErrPhotoNotFound
		}

		return tx.Model(&tag).Association("Photos").Append(&photos)
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
		if err := tx.Model(&photo).Association("Tags").Delete(&tag); err != nil {
			return err
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

		if err := tx.Model(&tag).Association("Photos").Clear(); err != nil {
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
