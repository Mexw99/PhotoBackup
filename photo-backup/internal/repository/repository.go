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
