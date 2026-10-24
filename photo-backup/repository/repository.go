package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"photo-backup/model"
)

var ErrEmptyTagName = errors.New("tag name is empty")

// Store is the database contract used by the service layer.
type Store interface {
	WithContext(context.Context) Store
	CreatePhoto(*model.Photo) error
	FindBackup(string, string) (*model.Photo, error)
	SaveBackup(*model.Photo) error
	GetSettings() (model.Setting, error)
	SaveSettings(string, string) error
	ListPhotos() ([]model.Photo, error)
	RecentPhotos(int) ([]model.Photo, error)
	CountPhotos() (int64, error)
	ListTags() ([]model.Tag, error)
	CountTags() (int64, error)
	AddTagToPhotos([]uint, string) (*model.Tag, error)
	RemoveTagFromPhoto(uint, uint) error
	DeleteTag(uint) error
	SearchPhotosByTags([]string) ([]model.Photo, error)
	SearchPhotos(string, []string) ([]model.Photo, error)
	GetTagCloud() ([]TagCount, error)
}

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

// WithContext creates an operation-scoped handle without mutating the shared DB.
func (r *Repository) WithContext(ctx context.Context) Store {
	return New(r.db.WithContext(ctx))
}

var _ Store = (*Repository)(nil)
