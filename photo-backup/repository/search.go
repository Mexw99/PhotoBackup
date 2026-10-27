package repository

import (
	"gorm.io/gorm"
	"strings"

	"photo-backup/model"
)

// เลือกหลาย Tag จะรวมรูปของทุก Tag ที่เลือก ไม่เลือกจะแสดงทั้งหมด
func (r *Repository) SearchPhotosByTags(tagNames []string) ([]model.Photo, error) {
	return r.SearchPhotos("", tagNames)
}

// SearchPhotos ใช้ชื่อไฟล์ร่วมกับแท็กที่เลือกได้
func (r *Repository) SearchPhotos(query string, tagNames []string) ([]model.Photo, error) {
	var photos []model.Photo
	err := r.photoSearch(query, tagNames).Preload("Tags").Order("backed_up_at DESC, id DESC").Find(&photos).Error
	return photos, err
}

type PhotoPage struct {
	Photos []model.Photo
	Total  int64
}

func (r *Repository) SearchPhotoPage(query string, tags []string, page, size int) (PhotoPage, error) {
	result := PhotoPage{Photos: []model.Photo{}}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		q := New(tx).photoSearch(query, tags)
		if err := q.Count(&result.Total).Error; err != nil {
			return err
		}
		return q.Preload("Tags").Order("backed_up_at DESC, id DESC").
			Offset((page - 1) * size).Limit(size).Find(&result.Photos).Error
	})
	return result, err
}

func (r *Repository) photoSearch(query string, tagNames []string) *gorm.DB {
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

	photoQuery := r.db.Model(&model.Photo{})
	if query = strings.TrimSpace(query); query != "" {
		literal := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(strings.ToLower(query))
		photoQuery = photoQuery.Where("LOWER(original_name) LIKE ? ESCAPE '\\'", "%"+literal+"%")
	}

	if len(lowerTags) > 0 {
		subQuery := r.db.Table("photo_tags").
			Select("photo_tags.photo_id").
			Joins("JOIN tags ON tags.id = photo_tags.tag_id").
			Where("LOWER(tags.name) IN ?", lowerTags)
		photoQuery = photoQuery.Where("photos.id IN (?)", subQuery)
	}
	return photoQuery
}
