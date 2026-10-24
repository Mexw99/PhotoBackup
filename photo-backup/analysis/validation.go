package analysis

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
)

// ValidateImage ตรวจว่าไฟล์มีนามสกุลรูปที่ Backup รองรับและมี header รูปภาพที่อ่านได้
// ใช้ DecodeConfig เพื่ออ่านเฉพาะข้อมูลขนาด/ชนิดภาพ ไม่โหลด pixel ทั้งไฟล์เข้า RAM
// ฟังก์ชันนี้จงใจเป็น validation อย่างเดียว ไม่แก้ไขไฟล์ต้นฉบับ
func ValidateImage(path string) error {
	extension := strings.ToLower(filepath.Ext(path))
	if extension != ".jpg" && extension != ".jpeg" && extension != ".png" {
		return fmt.Errorf("ไม่รองรับนามสกุลรูปภาพ: %s", extension)
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("เปิดไฟล์รูปภาพไม่ได้: %w", err)
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return fmt.Errorf("ไฟล์ไม่ใช่รูปภาพที่อ่านได้: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 {
		return fmt.Errorf("รูปภาพมีขนาดไม่ถูกต้อง")
	}
	return nil
}
