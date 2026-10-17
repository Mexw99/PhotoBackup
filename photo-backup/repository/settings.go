package repository

import (
	"errors"

	"gorm.io/gorm"

	"photo-backup/model"
)

func (r *Repository) GetSettings() (model.Setting, error) {
	var setting model.Setting
	err := r.db.Order("idx DESC").First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Setting{}, nil
	}
	return setting, err
}

func (r *Repository) SaveSettings(sourcePath, destPath string) error {
	setting, err := r.GetSettings()
	if err != nil {
		return err
	}
	setting.Source = sourcePath
	setting.Dest = destPath
	return r.db.Save(&setting).Error
}
