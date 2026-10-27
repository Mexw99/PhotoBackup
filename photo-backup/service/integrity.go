package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"photo-backup/analysis"
	"photo-backup/backup"
)

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
			Filename: photo.OriginalName, Path: photo.BackupPath, ExpectedSize: photo.FileSize, ActualSize: -1,
		}
		fileInfo, statErr := os.Lstat(photo.BackupPath)
		if os.IsNotExist(statErr) {
			result.Status = "ALERT"
			result.Message = "มีข้อมูลรูปนี้ใน Database แต่ไม่พบไฟล์ที่ปลายทาง"
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
			result.Message = "ตำแหน่งปลายทางไม่ใช่ไฟล์ปกติ"
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
		if photo.ContentHash != nil && strings.TrimSpace(*photo.ContentHash) != "" {
			diskHash, hashErr := analysis.SHA256File(photo.BackupPath)
			if hashErr != nil {
				result.Status = "ALERT"
				result.Message = "อ่านตรวจเนื้อหาไฟล์ปลายทางไม่ได้: " + hashErr.Error()
				results = append(results, result)
				continue
			}
			if diskHash != *photo.ContentHash {
				result.Status = "ALERT"
				result.Message = "เนื้อหาไฟล์ไม่ตรงกับ ContentHash ที่บันทึกใน Database"
				results = append(results, result)
				continue
			}
			result.Message = "ไฟล์และเนื้อหาตรงกับข้อมูลใน Database"
		} else {
			result.Message = "พบไฟล์และขนาดตรงกับ Database (รายการนี้ไม่มี ContentHash ให้ตรวจเนื้อหา)"
		}
		result.Status = "OK"
		results = append(results, result)
	}
	return results, nil
}
