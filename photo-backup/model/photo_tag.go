package model

const (
	TagSourceManual = "manual"
	TagSourceAI     = "ai"
)

// PhotoTag records where a relationship came from so a later AI re-analysis
// can replace only its own tags and preserve user-created tags.
type PhotoTag struct {
	PhotoID uint   `gorm:"primaryKey"`
	TagID   uint   `gorm:"primaryKey"`
	Source  string `gorm:"not null;default:'manual'"`
}
