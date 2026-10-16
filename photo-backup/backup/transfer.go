package backup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// BackupResult สรุปผลการคัดลอกรูปภาพแต่ละไฟล์
type BackupResult struct {
	OriginalName string `json:"originalName"`
	OriginalPath string `json:"originalPath"`
	BackupPath   string `json:"backupPath"`
	FileSize     int64  `json:"fileSize"`
	Success      bool   `json:"success"`
	Error        string `json:"error"`
}

// BackupSummary สรุปผลรวมของการ Backup ทั้งหมด
type BackupSummary struct {
	Total   int            `json:"total"`
	Success int            `json:"success"`
	Failed  int            `json:"failed"`
	Results []BackupResult `json:"results"`
}

// ProgressCallback ฟังก์ชันที่จะถูกเรียกทุกครั้งที่ copy เสร็จ 1 ไฟล์
// current = ไฟล์ที่ทำไปแล้ว, total = จำนวนไฟล์ทั้งหมด, name = ชื่อไฟล์ปัจจุบัน
type ProgressCallback func(current int, total int, name string)

// CopyFiles คัดลอกไฟล์รูปภาพจากรายการ images ไปยังโฟลเดอร์ destDir
// ป้องกันชื่อไฟล์ซ้ำโดยต่อ (1), (2), ... เข้าไปในชื่อไฟล์
// onProgress จะถูกเรียกทุกครั้งที่คัดลอกเสร็จ 1 ไฟล์ (ส่ง nil ได้ถ้าไม่ต้องการ)
func CopyFiles(images []ImageFile, destDir string, onProgress ProgressCallback) BackupSummary {
	summary := BackupSummary{
		Total:   len(images),
		Results: make([]BackupResult, 0, len(images)),
	}
	// สร้างโฟลเดอร์ปลายทางถ้ายังไม่มี
	if err := os.MkdirAll(destDir, 0755); err != nil {
		// ถ้าสร้างโฟลเดอร์ไม่ได้ → ทุกไฟล์ล้มเหลว
		for _, img := range images {
			summary.Failed++
			summary.Results = append(summary.Results, BackupResult{
				OriginalName: img.Name,
				OriginalPath: img.Path,
				Success:      false,
				Error:        "ไม่สามารถสร้างโฟลเดอร์ปลายทางได้: " + err.Error(),
			})
		}
		return summary
	}
	for i, img := range images {
		result := BackupResult{
			OriginalName: img.Name,
			OriginalPath: img.Path,
			FileSize:     img.Size,
		}
		// หาชื่อไฟล์ที่ไม่ซ้ำ
		destPath := uniqueDestPath(destDir, img.Name)
		// ย้ายไฟล์จากต้นทางไปปลายทาง
		if err := moveFile(img.Path, destPath); err != nil {
			result.Success = false
			result.Error = err.Error()
			summary.Failed++
		} else {
			result.Success = true
			result.BackupPath = destPath
			summary.Success++
		}
		summary.Results = append(summary.Results, result)
		// แจ้ง progress กลับไป
		if onProgress != nil {
			onProgress(i+1, len(images), img.Name)
		}
	}
	return summary
}

// uniqueDestPath สร้างชื่อไฟล์ที่ไม่ซ้ำในโฟลเดอร์ปลายทาง
// ตัวอย่าง: photo.jpg → photo.jpg, ถ้าซ้ำ → photo (1).jpg, photo (2).jpg, ...
func uniqueDestPath(destDir string, fileName string) string {
	destPath := filepath.Join(destDir, fileName)
	if _, err := os.Stat(destPath); errors.Is(err, os.ErrNotExist) {
		return destPath // ชื่อนี้ยังว่าง ใช้ได้เลย
	}
	ext := filepath.Ext(fileName)                 // ".jpg"
	nameOnly := strings.TrimSuffix(fileName, ext) // "photo"
	for i := 1; ; i++ {
		newName := fmt.Sprintf("%s (%d)%s", nameOnly, i, ext)
		newPath := filepath.Join(destDir, newName)
		if _, err := os.Stat(newPath); errors.Is(err, os.ErrNotExist) {
			return newPath
		}
	}
}

// moveFile ย้ายไฟล์จาก src ไปยัง dst (รองรับทั้งไดรฟ์เดียวกัน และข้ามไดรฟ์ เช่น Flash Drive ไป C:)
func moveFile(src string, dst string) error {
	// 1. ลอง os.Rename ก่อน (เร็วมากถ้าอยู่ในไดรฟ์เดียวกัน)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// 2. ถ้าข้ามไดรฟ์ (เช่น D: ไป C:) ให้ Copy ไปปลายทางก่อน
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("เปิดไฟล์ต้นทางไม่ได้: %w", err)
	}

	destFile, err := os.Create(dst)
	if err != nil {
		sourceFile.Close()
		return fmt.Errorf("สร้างไฟล์ปลายทางไม่ได้: %w", err)
	}

	_, copyErr := io.Copy(destFile, sourceFile)
	// ต้องปิด handle ทั้ง 2 ไฟล์ทันที เพื่อไม่ให้ Windows ล็อคไฟล์ตอนลบ
	sourceFile.Close()
	destFile.Close()

	if copyErr != nil {
		_ = os.Remove(dst) // ลบไฟล์ปลายทางที่เขียนไม่เสร็จทิ้ง
		return fmt.Errorf("คัดลอกข้อมูลไม่สำเร็จ: %w", copyErr)
	}

	// 3. ลบไฟล์ต้นทางออก เพื่อให้เป็นการ "ย้าย" จริงๆ
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("ย้ายไฟล์สำเร็จแต่ลบไฟล์ต้นทางไม่ได้: %w", err)
	}

	return nil
}
