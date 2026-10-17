package repository

import "photo-backup/model"

func (r *Repository) CreatePhoto(photo *model.Photo) error {
	return r.db.Create(photo).Error
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
