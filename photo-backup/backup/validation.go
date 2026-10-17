package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateFolders ตรวจโฟลเดอร์ก่อนเริ่มงาน เพื่อไม่สำรองวนเข้าต้นทางตัวเอง
func ValidateFolders(sourcePath, destPath string) error {
	if strings.TrimSpace(sourcePath) == "" || strings.TrimSpace(destPath) == "" {
		return fmt.Errorf("กรุณาเลือกโฟลเดอร์ต้นทางและปลายทาง")
	}
	source, err := filepath.Abs(sourcePath)
	if err != nil {
		return err
	}
	dest, err := filepath.Abs(destPath)
	if err != nil {
		return err
	}
	info, err := os.Stat(source)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("เปิดโฟลเดอร์ต้นทางไม่ได้ กรุณาตรวจการเชื่อมต่ออุปกรณ์")
	}
	if foldersOverlap(source, dest) {
		return fmt.Errorf("ต้นทางและปลายทางต้องเป็นคนละโฟลเดอร์และไม่ซ้อนกัน")
	}
	if err := os.MkdirAll(dest, 0755); err != nil {
		return fmt.Errorf("สร้างโฟลเดอร์ปลายทางไม่ได้: %w", err)
	}
	// ตรวจอีกครั้งสำหรับโฟลเดอร์ที่เชื่อมผ่าน symbolic link
	source, err = filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	dest, err = filepath.EvalSymlinks(dest)
	if err != nil {
		return err
	}
	if foldersOverlap(source, dest) {
		return fmt.Errorf("ต้นทางและปลายทางต้องเป็นคนละโฟลเดอร์และไม่ซ้อนกัน")
	}
	return nil
}

func foldersOverlap(first, second string) bool {
	return insideFolder(first, second) || insideFolder(second, first)
}

func insideFolder(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
