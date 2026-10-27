package backup

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
)

// ValidateImage checks the image header without decoding pixels or analyzing
// colors. It keeps invalid files from being copied into the photo library.
func ValidateImage(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, _, err := image.DecodeConfig(file); err != nil {
		return fmt.Errorf("ไฟล์รูปภาพไม่ถูกต้อง: %w", err)
	}
	return nil
}

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

func ValidateDestinationPath(root, directory string) error {
	if !insideFolder(root, directory) {
		return fmt.Errorf("ปลายทางอยู่นอกโฟลเดอร์ที่เลือก")
	}
	relative, err := filepath.Rel(root, directory)
	if err != nil {
		return err
	}
	current := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("โฟลเดอร์ปลายทางย่อยเป็นลิงก์หรือไม่ใช่โฟลเดอร์: %s", current)
		}
	}
	return nil
}
