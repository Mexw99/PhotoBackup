package model

import "time"

// AIUsage นับคำขอที่ส่งไป Provider จริงในแต่ละวันแบบถาวรใน SQLite
type AIUsage struct {
	ID           uint   `gorm:"primaryKey"`
	UsageDate    string `gorm:"not null;uniqueIndex:idx_ai_usage_day_provider_model,priority:1"`
	Provider     string `gorm:"not null;uniqueIndex:idx_ai_usage_day_provider_model,priority:2"`
	Model        string `gorm:"not null;uniqueIndex:idx_ai_usage_day_provider_model,priority:3"`
	RequestCount int    `gorm:"not null;default:0"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
