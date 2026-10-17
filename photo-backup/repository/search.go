package repository

import (
	"strings"

	"photo-backup/model"
)

// เลือกหลาย Tag จะรวมรูปของทุก Tag ที่เลือก ไม่เลือกจะแสดงทั้งหมด
func (r *Repository) SearchPhotosByTags(tagNames []string) ([]model.Photo, error) {
	return r.SearchPhotos("", tagNames)
}

// SearchPhotos ใช้ชื่อไฟล์ร่วมกับแท็กที่เลือกได้
func (r *Repository) SearchPhotos(query string, tagNames []string) ([]model.Photo, error) {
	// ตัดแท็กซ้ำ/ว่าง และเทียบชื่อโดยไม่แยกตัวพิมพ์ใหญ่เล็ก
	uniqueMap := make(map[string]bool)
	var lowerTags []string

	for _, name := range tagNames {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if !uniqueMap[lower] {
			uniqueMap[lower] = true
			lowerTags = append(lowerTags, lower)
		}
	}

	var photos []model.Photo
	photoQuery := r.db.Preload("Tags").Order("backed_up_at DESC")
	if query = strings.TrimSpace(query); query != "" {
		pattern := "%" + strings.ToLower(query) + "%"
		photoQuery = photoQuery.Where("LOWER(original_name) LIKE ?", pattern)
	}

	if len(lowerTags) > 0 {
		subQuery := r.db.Table("photo_tags").
			Select("photo_tags.photo_id").
			Joins("JOIN tags ON tags.id = photo_tags.tag_id").
			Where("LOWER(tags.name) IN ?", lowerTags)
		photoQuery = photoQuery.Where("photos.id IN (?)", subQuery)
	}
	err := photoQuery.Find(&photos).Error
	return photos, err
}
