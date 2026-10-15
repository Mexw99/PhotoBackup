package repository

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"photo-backup/internal/models"
)

var ErrEmptyTagName = errors.New("tag name is empty")

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) CreatePhoto(photo *models.Photo) error {
	return r.db.Create(photo).Error
}

func (r *Repository) ListPhotos() ([]models.Photo, error) {
	var photos []models.Photo

	err := r.db.
		Preload("Tags").
		Order("backed_up_at DESC").
		Find(&photos).
		Error

	return photos, err
}

func (r *Repository) RecentPhotos(limit int) ([]models.Photo, error) {
	if limit <= 0 {
		limit = 5
	}

	var photos []models.Photo

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
		Model(&models.Photo{}).
		Count(&count).
		Error

	return count, err
}

func (r *Repository) CreateTag(name string) (*models.Tag, error) {
	name = strings.TrimSpace(name)

	if name == "" {
		return nil, ErrEmptyTagName
	}

	tag := &models.Tag{
		Name: name,
	}

	err := r.db.
		Where("name = ?", name).
		FirstOrCreate(tag).
		Error

	return tag, err
}

func (r *Repository) ListTags() ([]models.Tag, error) {
	var tags []models.Tag

	err := r.db.
		Order("name ASC").
		Find(&tags).
		Error

	return tags, err
}

func (r *Repository) CountTags() (int64, error) {
	var count int64

	err := r.db.
		Model(&models.Tag{}).
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
func (r *Repository) AddTagToPhotos(photoIDs []uint, name string) (*models.Tag, error) {
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

	var tag models.Tag

	err := r.db.Transaction(func(tx *gorm.DB) error {
		// FirstOrCreate ป้องกันการสร้าง Tag ชื่อซ้ำ
		if err := tx.Where("name = ?", name).
			FirstOrCreate(&tag, models.Tag{Name: name}).Error; err != nil {
			return err
		}

		var photos []models.Photo
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
	var photo models.Photo
	if err := r.db.First(&photo, photoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPhotoNotFound
		}
		return err
	}

	var tag models.Tag
	if err := r.db.First(&tag, tagID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTagNotFound
		}
		return err
	}

	return r.db.Model(&photo).Association("Tags").Delete(&tag)
}

// คืน Tag พร้อมจำนวนรูป เรียงจำนวนมากไปน้อย
func (r *Repository) GetTagCloud() ([]TagCount, error) {
	var result []TagCount

	err := r.db.Model(&models.Tag{}).
		Select("tags.id, tags.name, COUNT(DISTINCT photo_tags.photo_id) AS photo_count").
		Joins("LEFT JOIN photo_tags ON photo_tags.tag_id = tags.id").
		Group("tags.id, tags.name").
		Order("photo_count DESC").
		Order("tags.name ASC").
		Scan(&result).Error

	return result, err
}
