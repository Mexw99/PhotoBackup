package service

import (
	"path/filepath"
	"strings"

	"photo-backup/backup"
	"photo-backup/model"
)

func (s *Service) GetBackupSettings() (model.Setting, error) {
	store, cancel := s.operation()
	defer cancel()
	return store.GetSettings()
}

func (s *Service) SaveBackupSettings(sourcePath, destPath string) error {
	sourcePath = strings.TrimSpace(sourcePath)
	destPath = strings.TrimSpace(destPath)
	if err := backup.ValidateFolders(sourcePath, destPath); err != nil {
		return err
	}
	// เก็บ path เต็ม เพื่อไม่ให้เปลี่ยนตามโฟลเดอร์ที่เปิดโปรแกรม
	sourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return err
	}
	destPath, err = filepath.Abs(destPath)
	if err != nil {
		return err
	}
	store, cancel := s.operation()
	defer cancel()
	return store.SaveSettings(sourcePath, destPath)
}
