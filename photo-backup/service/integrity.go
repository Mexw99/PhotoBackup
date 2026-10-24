package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"photo-backup/backup"
)

// CheckBackup compares saved photo records with the files currently on disk.
// It follows the midterm behavior: report missing files and size mismatches
// without deleting or changing anything.
func (s *Service) CheckBackup(destPath string) ([]backup.IntegrityResult, error) {
	destPath = strings.TrimSpace(destPath)
	if destPath == "" {
		return nil, fmt.Errorf("กรุณาเลือกโฟลเดอร์ปลายทางก่อนตรวจสอบ")
	}
	absDest, err := filepath.Abs(destPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absDest)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("เปิดโฟลเดอร์ปลายทางไม่ได้ กรุณาตรวจสอบ Flash Drive หรือโฟลเดอร์ที่เลือก")
	}

	store, cancel := s.operation()
	photos, err := store.PhotosInDestination(absDest)
	cancel()
	if err != nil {
		return nil, err
	}

	results := make([]backup.IntegrityResult, 0, len(photos))
	for _, photo := range photos {
		result := backup.IntegrityResult{
			Filename:     photo.OriginalName,
			Path:         photo.BackupPath,
			ExpectedSize: photo.FileSize,
		}
		fileInfo, statErr := os.Lstat(photo.BackupPath)
		if os.IsNotExist(statErr) {
			result.Status = "ALERT"
			result.Message = "มีข้อมูลใน Database แต่ไม่พบไฟล์ที่ปลายทาง"
			results = append(results, result)
			continue
		}
		if statErr != nil {
			result.Status = "ALERT"
			result.Message = "ตรวจสอบไฟล์ปลายทางไม่ได้: " + statErr.Error()
			results = append(results, result)
			continue
		}
		if !fileInfo.Mode().IsRegular() {
			result.Status = "ALERT"
			result.Message = "ตำแหน่งปลายทางไม่ใช่ไฟล์รูปภาพปกติ"
			results = append(results, result)
			continue
		}
		result.ActualSize = fileInfo.Size()
		if result.ActualSize != result.ExpectedSize {
			result.Status = "ALERT"
			result.Message = fmt.Sprintf("ขนาดไฟล์ไม่ตรงกัน: Database %d bytes, Disk %d bytes", result.ExpectedSize, result.ActualSize)
			results = append(results, result)
			continue
		}
		result.Status = "OK"
		result.Message = "ไฟล์ตรงกับข้อมูลใน Database"
		results = append(results, result)
	}
	return results, nil
}
