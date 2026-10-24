package model

import "time"

type Photo struct {
	ID               uint   `gorm:"primaryKey"`
	OriginalName     string `gorm:"not null"`
	OriginalPath     string `gorm:"not null;index"`
	BackupPath       string `gorm:"not null;uniqueIndex"`
	FileSize         int64  `gorm:"not null;default:0"`
	ColorTag         string `gorm:"not null;default:''"`
	SourceModifiedAt time.Time
	BackedUpAt       time.Time `gorm:"not null"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Tags             []Tag `gorm:"many2many:photo_tags;"`
}
