package model

import "time"

// AIResult stores a reusable analysis result keyed by image content and the
// analysis configuration that produced it.
type AIResult struct {
	ID              uint   `gorm:"primaryKey"`
	ContentHash     string `gorm:"type:char(64);not null;index:idx_ai_results_cache_key,unique,priority:1"`
	AnalysisProfile string `gorm:"not null;index:idx_ai_results_cache_key,unique,priority:2"`
	Provider        string `gorm:"not null"`
	Model           string `gorm:"not null"`
	PromptVersion   string `gorm:"not null;index:idx_ai_results_cache_key,unique,priority:3"`
	Description     string `gorm:"type:text;not null;default:''"`
	TagsJSON        string `gorm:"type:text;not null;default:'[]'"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
