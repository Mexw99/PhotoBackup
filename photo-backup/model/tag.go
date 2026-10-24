package model

import "time"

type Tag struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"not null;uniqueIndex"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Photos    []Photo `gorm:"many2many:photo_tags;"`
}

// PhotoTag keeps the origin of a relationship separate from the tag name.
// Existing rows are migrated as user-created tags by the database default.
type PhotoTag struct {
	PhotoID uint   `gorm:"primaryKey"`
	TagID   uint   `gorm:"primaryKey"`
	Source  string `gorm:"primaryKey;not null;default:'user';index"`
}

func (PhotoTag) TableName() string { return "photo_tags" }
