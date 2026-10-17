package model

// FileRecord เก็บประวัติสำรองในรูปแบบตาราง files ที่ใช้ตั้งแต่กลางภาค
type FileRecord struct {
	Idx      uint   `gorm:"primaryKey;autoIncrement"`
	Dest     string `gorm:"not null"`
	Filename string `gorm:"not null"`
	Status   string `gorm:"not null"`
}

func (FileRecord) TableName() string { return "files" }
