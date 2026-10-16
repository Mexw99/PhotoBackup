package model

import "time"

type Tag struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"not null;uniqueIndex"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Photos    []Photo `gorm:"many2many:photo_tags;"`
}
