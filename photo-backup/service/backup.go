package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"photo-backup/analysis"
	"photo-backup/backup"
	"photo-backup/model"
)

// ป้องกันกดสำรองซ้อนกันระหว่างที่ยังทำงานกับไฟล์ชุดเดิม
var backupLock sync.Mutex

func ListImageFiles(sourcePath string) ([]backup.ImageFile, error) {
	return backup.DiscoverImages(sourcePath)
}

func DefaultBackupFolder() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "PhotoBackup")
}

// StartBackup: ตรวจโฟลเดอร์ → ดูสี → เตรียมปลายทาง → บันทึก DB → ลบต้นทาง
func (s *Service) StartBackup(sourcePath, destPath string, onProgress backup.ProgressCallback) (backup.BackupSummary, error) {
	if !backupLock.TryLock() {
		return backup.BackupSummary{}, fmt.Errorf("กำลังสำรองรูปภาพอยู่ กรุณารอให้เสร็จก่อน")
	}
	defer backupLock.Unlock()
	if err := backup.ValidateFolders(sourcePath, destPath); err != nil {
		return backup.BackupSummary{}, err
	}
	sourcePath, _ = filepath.Abs(sourcePath)
	destPath, _ = filepath.Abs(destPath)
	images, err := backup.DiscoverImages(sourcePath)
	if err != nil {
		return backup.BackupSummary{}, err
	}
	if len(images) == 0 {
		return backup.BackupSummary{Results: []backup.BackupResult{}}, nil
	}
	if err := s.SaveBackupSettings(sourcePath, destPath); err != nil {
		return backup.BackupSummary{}, err
	}

	summary := backup.BackupSummary{Total: len(images), Results: make([]backup.BackupResult, 0, len(images))}
	for index, image := range images {
		result, err := s.backupPhoto(image, sourcePath, destPath)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
			summary.Failed++
		} else if result.Skipped {
			summary.Skipped++
		} else {
			summary.Success++
		}
		summary.Results = append(summary.Results, result)
		if onProgress != nil {
			onProgress(index+1, len(images), image.Name)
		}
	}
	return summary, nil
}

func (s *Service) backupPhoto(image backup.ImageFile, sourcePath, destPath string) (backup.BackupResult, error) {
	result := backup.BackupResult{OriginalName: image.Name, OriginalPath: image.Path, FileSize: image.Size}
	info, err := os.Stat(image.Path)
	if err != nil {
		return result, err
	}
	image.Size, result.FileSize = info.Size(), info.Size()
	store, cancel := s.operation()
	previous, err := store.FindBackup(image.Path, destPath)
	cancel()
	if err != nil {
		return result, err
	}
	if alreadyBackedUp(previous, info, destPath) {
		equal, err := backup.EqualFiles(image.Path, previous.BackupPath)
		if err != nil {
			return result, err
		}
		if equal {
			result.BackupPath = previous.BackupPath
			if err := s.finishMove(image.Path, result.BackupPath, info); err != nil {
				return result, err
			}
			result.Success, result.Skipped = true, true
			return result, nil
		}
	}

	colorTag, err := analysis.DetectColor(image.Path)
	if err != nil {
		return result, err
	}
	relative, err := filepath.Rel(sourcePath, image.Path)
	if err != nil {
		return result, err
	}
	result, err = backup.CopyImage(image, filepath.Join(destPath, filepath.Dir(relative)))
	if err != nil {
		return result, err
	}
	photo := &model.Photo{
		OriginalName:     image.Name,
		OriginalPath:     image.Path,
		BackupPath:       result.BackupPath,
		FileSize:         image.Size,
		SourceModifiedAt: info.ModTime(),
		BackedUpAt:       time.Now(),
		ColorTag:         colorTag,
	}
	store, cancel = s.operation()
	err = store.SaveBackup(photo)
	cancel()
	if err != nil {
		// หาก DB บันทึกไม่ได้ ลบเฉพาะสำเนาที่เพิ่งสร้าง และเก็บต้นฉบับไว้
		_ = os.Remove(result.BackupPath)
		result.BackupPath = ""
		return result, fmt.Errorf("บันทึกข้อมูลรูปไม่ได้: %w", err)
	}
	if err := s.finishMove(image.Path, result.BackupPath, info); err != nil {
		return result, err
	}
	return result, nil
}

// ลบต้นทางหลังปลายทางและ DB พร้อมแล้วเท่านั้น
func (s *Service) finishMove(path, backupPath string, expected os.FileInfo) error {
	destination, err := os.Lstat(backupPath)
	if err != nil {
		return fmt.Errorf("ตรวจสอบปลายทางก่อนลบต้นทางไม่ได้: %w", err)
	}
	if !destination.Mode().IsRegular() || destination.Size() != expected.Size() {
		return fmt.Errorf("ปลายทางเปลี่ยนระหว่างย้าย จึงยังไม่ลบต้นทาง")
	}
	current, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !os.SameFile(expected, current) || expected.Size() != current.Size() || !expected.ModTime().Equal(current.ModTime()) {
		return fmt.Errorf("ไฟล์ต้นทางเปลี่ยนระหว่างย้าย จึงยังไม่ลบต้นทาง กรุณาลองใหม่")
	}
	if err := s.removeSource(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("ปลายทางและฐานข้อมูลพร้อมแล้ว แต่ลบต้นทางไม่ได้ กรุณาลองใหม่: %w", err)
	}
	return nil
}

// กรองประวัติด้วย path/ขนาด/เวลาก่อนเทียบ bytes จริงและย้ายให้จบ
func alreadyBackedUp(photo *model.Photo, source os.FileInfo, destPath string) bool {
	if photo == nil || photo.FileSize != source.Size() || !photo.SourceModifiedAt.Equal(source.ModTime()) {
		return false
	}
	relative, err := filepath.Rel(destPath, photo.BackupPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	destination, err := os.Lstat(photo.BackupPath)
	return err == nil && destination.Mode().IsRegular() && destination.Size() == photo.FileSize
}
