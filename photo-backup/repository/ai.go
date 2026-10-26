package repository

import (
	"errors"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"photo-backup/model"
)

var aiJobClaimLock sync.Mutex

func (r *Repository) FindPhoto(id uint) (*model.Photo, error) {
	var photo model.Photo
	if err := r.db.First(&photo, id).Error; err != nil {
		return nil, err
	}
	return &photo, nil
}

func (r *Repository) UpdatePhotoContentHash(id uint, hash *string) error {
	return r.db.Model(&model.Photo{}).Where("id = ?", id).Update("content_hash", hash).Error
}

func (r *Repository) FindAIResult(contentHash, profile, promptVersion string) (*model.AIResult, error) {
	var result model.AIResult
	err := r.db.Where("content_hash = ? AND analysis_profile = ? AND prompt_version = ?", contentHash, profile, promptVersion).First(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *Repository) HasAITags(photoID uint) (bool, error) {
	var count int64
	err := r.db.Model(&model.PhotoTag{}).
		Where("photo_id = ? AND source = ?", photoID, "ai").
		Count(&count).Error
	return count > 0, err
}

// SaveAIResultAndApply ใช้ transaction เดียวทั้ง Cache และ Photo/Tag
// Provider กับ Model เป็นข้อมูลอ้างอิง จึงไม่อยู่ใน unique cache key
func (r *Repository) SaveAIResultAndApply(result *model.AIResult, tagNames []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "content_hash"}, {Name: "analysis_profile"}, {Name: "prompt_version"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"provider", "model", "description", "tags_json", "updated_at"}),
		}).Create(result).Error; err != nil {
			return err
		}

		var photos []model.Photo
		if err := tx.Where("content_hash = ?", result.ContentHash).Find(&photos).Error; err != nil {
			return err
		}
		for index := range photos {
			if err := tx.Where("photo_id = ? AND source = ?", photos[index].ID, "ai").Delete(&model.PhotoTag{}).Error; err != nil {
				return err
			}
			if err := tx.Model(&photos[index]).Update("description", strings.TrimSpace(result.Description)).Error; err != nil {
				return err
			}
			seen := make(map[string]bool)
			for _, name := range tagNames {
				name = normalizeTagName(name)
				key := strings.ToLower(name)
				if name == "" || seen[key] {
					continue
				}
				seen[key] = true
				tag, err := findOrCreateTag(tx, name)
				if err != nil {
					return err
				}
				if err := appendPhotoTag(tx, photos[index].ID, tag.ID, "ai"); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *Repository) CreateAIJobIfMissing(job *model.AIJob) (*model.AIJob, error) {
	if job == nil {
		return nil, errors.New("AI job is nil")
	}
	if job.Status == "" {
		job.Status = "queued"
	}
	var existing model.AIJob
	err := r.db.Where("photo_id = ? AND analysis_profile = ? AND prompt_version = ?", job.PhotoID, job.AnalysisProfile, job.PromptVersion).First(&existing).Error
	if err == nil {
		// Refresh stale or incomplete jobs so a previous failure cannot block a
		// repaired worker or leave a photo without AI tags.
		if existing.Status == "failed" || existing.Status == "completed" || existing.ContentHash != job.ContentHash {
			updates := map[string]interface{}{
				"content_hash": job.ContentHash, "provider": job.Provider, "model": job.Model,
				"status": "queued", "attempt_count": 0, "last_error": "",
				"next_retry_at": nil, "started_at": nil, "finished_at": nil,
			}
			if err := r.db.Model(&existing).Updates(updates).Error; err != nil {
				return nil, err
			}
			return &existing, r.db.First(&existing, existing.ID).Error
		}
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err := r.db.Create(job).Error; err != nil {
		// อีก worker อาจสร้าง Job คีย์เดียวกันไปก่อนแล้ว ให้ใช้แถวนั้นแทน
		if lookupErr := r.db.Where("photo_id = ? AND analysis_profile = ? AND prompt_version = ?", job.PhotoID, job.AnalysisProfile, job.PromptVersion).First(&existing).Error; lookupErr == nil {
			return &existing, nil
		}
		return nil, err
	}
	return job, nil
}

func (r *Repository) ResetProcessingAIJobs() error {
	return r.db.Model(&model.AIJob{}).Where("status = ?", "processing").Updates(map[string]interface{}{
		"status": "queued", "started_at": nil, "last_error": "พักงานระหว่างปิดโปรแกรม จัดคิวใหม่เมื่อเปิดโปรแกรม",
	}).Error
}

func (r *Repository) ClaimNextAIJob(now time.Time) (*model.AIJob, error) {
	aiJobClaimLock.Lock()
	defer aiJobClaimLock.Unlock()

	var job model.AIJob
	err := r.db.Transaction(func(tx *gorm.DB) error {
		query := tx.Where("status = ?", "queued").Or("status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?)", "retry_wait", now).
			Order("created_at ASC").Order("id ASC").First(&job)
		if errors.Is(query.Error, gorm.ErrRecordNotFound) {
			return gorm.ErrRecordNotFound
		}
		if query.Error != nil {
			return query.Error
		}
		updates := map[string]interface{}{
			"status": "processing", "attempt_count": gorm.Expr("attempt_count + 1"),
			"started_at": now, "finished_at": nil, "last_error": "",
		}
		if err := tx.Model(&model.AIJob{}).Where("id = ? AND status IN ?", job.ID, []string{"queued", "retry_wait"}).Updates(updates).Error; err != nil {
			return err
		}
		return tx.First(&job, job.ID).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *Repository) UpdateAIJob(job *model.AIJob) error {
	return r.db.Save(job).Error
}

func (r *Repository) AIJobCounts() (map[string]int64, error) {
	counts := make(map[string]int64)
	var rows []struct {
		Status string
		Count  int64
	}
	if err := r.db.Model(&model.AIJob{}).Select("status, COUNT(*) AS count").Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.Status] = row.Count
	}
	return counts, nil
}

// ConsumeAIRequest จองโควตาก่อนเรียก Provider จริง; limit <= 0 หมายถึงไม่จำกัด
func (r *Repository) ConsumeAIRequest(provider, modelName string, limit int) (bool, error) {
	if limit <= 0 {
		return true, nil
	}
	date := time.Now().Format("2006-01-02")
	allowed := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var usage model.AIUsage
		err := tx.Where("usage_date = ? AND provider = ? AND model = ?", date, provider, modelName).First(&usage).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			usage = model.AIUsage{UsageDate: date, Provider: provider, Model: modelName, RequestCount: 1}
			if err := tx.Create(&usage).Error; err != nil {
				return err
			}
			allowed = true
			return nil
		}
		if err != nil {
			return err
		}
		if usage.RequestCount >= limit {
			return nil
		}
		if err := tx.Model(&usage).Update("request_count", gorm.Expr("request_count + 1")).Error; err != nil {
			return err
		}
		allowed = true
		return nil
	})
	return allowed, err
}
