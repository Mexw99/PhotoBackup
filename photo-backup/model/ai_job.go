package model

import "time"

// AIJob เป็นงานวิเคราะห์ที่เก็บถาวร เพื่อให้เปิดโปรแกรมใหม่แล้วทำงานต่อได้
type AIJob struct {
	ID              uint   `gorm:"primaryKey"`
	PhotoID         uint   `gorm:"not null;index;uniqueIndex:idx_ai_jobs_photo_profile_prompt,priority:1"`
	ContentHash     string `gorm:"type:char(64);not null;index"`
	AnalysisProfile string `gorm:"not null;uniqueIndex:idx_ai_jobs_photo_profile_prompt,priority:2"`
	PromptVersion   string `gorm:"not null;uniqueIndex:idx_ai_jobs_photo_profile_prompt,priority:3"`
	Provider        string `gorm:"not null;default:''"`
	Model           string `gorm:"not null;default:''"`
	Status          string `gorm:"not null;index"`
	AttemptCount    int    `gorm:"not null;default:0"`
	LastError       string `gorm:"type:text;not null;default:''"`
	NextRetryAt     *time.Time
	StartedAt       *time.Time
	FinishedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
