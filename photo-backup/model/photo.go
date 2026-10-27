package model

import "time"

type Photo struct {
	ID               uint   `gorm:"primaryKey"`
	OriginalName     string `gorm:"not null"`
	OriginalPath     string `gorm:"not null;index"`
	BackupPath       string `gorm:"not null;uniqueIndex"`
	FileSize         int64  `gorm:"not null;default:0"`
	Description      string `gorm:"not null;default:''"`
	AnalysisStatus   string `gorm:"not null;default:'pending';index"`
	AnalysisError    string `gorm:"not null;default:''"`
	ContentHash      string `gorm:"not null;default:'';index"`
	AnalyzedAt       *time.Time
	SourceModifiedAt time.Time
	BackedUpAt       time.Time `gorm:"not null"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Tags             []Tag `gorm:"many2many:photo_tags;"`
}

const (
	AnalysisPending    = "pending"
	AnalysisProcessing = "processing"
	AnalysisRetrying   = "retrying"
	AnalysisCompleted  = "completed"
	AnalysisFailed     = "failed"
)
