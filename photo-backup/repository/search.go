package repository

import (
	"photo-backup/model"
	"strings"
)

// SearchPhotosByTags ค้นหารูปภาพตามชื่อ Tag ด้วยโหมด "AND" หรือ "OR"
// ป้องกัน SQL Injection โดยใช้ parameterized queries และ GORM methods
func (r *Repository) SearchPhotosByTags(tagNames []string, mode string) ([]model.Photo, error) {
	// 1. ทำความสะอาดและกรอง Tag ที่ซ้ำ/ว่าง
	uniqueMap := make(map[string]bool)
	var cleanTags []string
	var lowerTags []string

	for _, name := range tagNames {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if !uniqueMap[lower] {
			uniqueMap[lower] = true
			cleanTags = append(cleanTags, trimmed)
			lowerTags = append(lowerTags, lower)
		}
	}

	// หากไม่มี Tag ที่ส่งมา ให้คืนรายการรูปภาพทั้งหมด
	if len(cleanTags) == 0 {
		return r.ListPhotos()
	}

	normalizedMode := strings.ToUpper(strings.TrimSpace(mode))
	if normalizedMode != "AND" {
		normalizedMode = "OR"
	}

	var photos []model.Photo

	if normalizedMode == "AND" {
		// โหมด AND: รูปภาพต้องมี Tag ครบทุกรายการที่เลือก
		subQuery := r.db.Model(&model.Photo{}).
			Select("photos.id").
			Joins("JOIN photo_tags ON photo_tags.photo_id = photos.id").
			Joins("JOIN tags ON tags.id = photo_tags.tag_id").
			Where("LOWER(tags.name) IN ?", lowerTags).
			Group("photos.id").
			Having("COUNT(DISTINCT LOWER(tags.name)) = ?", len(lowerTags))

		err := r.db.
			Preload("Tags").
			Where("id IN (?)", subQuery).
			Order("backed_up_at DESC").
			Find(&photos).
			Error

		return photos, err
	}

	// โหมด OR: รูปภาพต้องมี Tag อย่างน้อยหนึ่งรายการที่เลือก
	subQuery := r.db.Model(&model.Photo{}).
		Select("photos.id").
		Joins("JOIN photo_tags ON photo_tags.photo_id = photos.id").
		Joins("JOIN tags ON tags.id = photo_tags.tag_id").
		Where("LOWER(tags.name) IN ?", lowerTags).
		Distinct()

	err := r.db.
		Preload("Tags").
		Where("id IN (?)", subQuery).
		Order("backed_up_at DESC").
		Find(&photos).
		Error

	return photos, err
}
