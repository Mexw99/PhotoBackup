package service

import (
	"os"
	"strings"

	"photo-backup/analysis"
)

// BackfillContentHashes เติม Hash เฉพาะ Photo ที่ยังไม่มีค่า โดยอ่าน BackupPath
// เท่านั้น ไม่เรียก AI และไม่ถูกเรียกอัตโนมัติตอนเปิดโปรแกรม
// limit <= 0 ใช้ค่าเริ่มต้น 100 เพื่อไม่ให้การเปิดแอปค้างกับฐานข้อมูลขนาดใหญ่
func (s *Service) BackfillContentHashes(limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	store, cancel := s.operation()
	photos, err := store.ListPhotos()
	cancel()
	if err != nil {
		return 0, err
	}
	updated := 0
	for _, photo := range photos {
		if updated >= limit || photo.ContentHash != nil || strings.TrimSpace(photo.BackupPath) == "" {
			continue
		}
		info, statErr := os.Stat(photo.BackupPath)
		if statErr != nil || !info.Mode().IsRegular() {
			continue
		}
		hash, hashErr := analysis.SHA256File(photo.BackupPath)
		if hashErr != nil {
			continue
		}
		store, cancel = s.operation()
		updateErr := store.UpdatePhotoContentHash(photo.ID, &hash)
		cancel()
		if updateErr != nil {
			return updated, updateErr
		}
		updated++
	}
	return updated, nil
}
