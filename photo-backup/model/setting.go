package model

// Setting เก็บโฟลเดอร์ที่เลือกไว้ เพื่อใช้ต่อเมื่อเปิดโปรแกรมครั้งหน้า
type Setting struct {
	Idx    uint   `gorm:"primaryKey;autoIncrement"`
	Source string `gorm:"not null"`
	Dest   string `gorm:"not null"`
}

func (Setting) TableName() string { return "settings" }
