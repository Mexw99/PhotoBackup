package service

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"photo-backup/analysis"
	"photo-backup/backup"
	"photo-backup/model"
)

// ป้องกันกดสำรองซ้อนกันระหว่างที่ยังทำงานกับไฟล์ชุดเดิม
var backupLock sync.Mutex

// SQLite เขียน transaction พร้อมกันหลายตัวได้ไม่ดีนัก โดยเฉพาะตอนใช้กับ
// Flash Drive จึงล็อกเฉพาะช่วง SaveBackup ส่วนการอ่าน/วิเคราะห์/คัดลอกยังทำพร้อมกันได้
var backupDatabaseLock sync.Mutex

type backupOutcome struct {
	index  int
	image  backup.ImageFile
	result backup.BackupResult
	err    error
}

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

// StartBackup: ตรวจโฟลเดอร์ → ตรวจไฟล์รูป → คัดลอก → verify → hash/บันทึก DB → สร้าง AI Job → ลบต้นทาง
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
	duplicatePaths, err := s.backupPathsInDestination(destPath)
	if err != nil {
		return backup.BackupSummary{}, err
	}

	summary := backup.BackupSummary{
		Total:   len(images),
		Results: make([]backup.BackupResult, len(images)),
	}

	// ทำงานพร้อมกันจำนวนจำกัด เพื่อไม่เปิดไฟล์/ใช้ดิสก์มากเกินไป
	workerCount := runtime.NumCPU()
	if workerCount < 2 {
		workerCount = 2
	}
	if workerCount > 4 {
		workerCount = 4
	}
	if workerCount > len(images) {
		workerCount = len(images)
	}

	jobs := make(chan int)
	outcomes := make(chan backupOutcome, workerCount)
	var workers sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				image := images[index]
				result, err := s.backupPhoto(image, sourcePath, destPath, duplicatePaths)
				outcomes <- backupOutcome{index: index, image: image, result: result, err: err}
			}
		}()
	}

	go func() {
		for index := range images {
			jobs <- index
		}
		close(jobs)
		workers.Wait()
		close(outcomes)
	}()

	completed := 0
	for outcome := range outcomes {
		result := outcome.result
		if outcome.err != nil {
			result.Success = false
			result.Error = outcome.err.Error()
			summary.Failed++
		} else if result.Skipped {
			summary.Skipped++
		} else {
			summary.Success++
		}
		summary.Results[outcome.index] = result
		completed++
		if onProgress != nil {
			onProgress(completed, len(images), outcome.image.Name)
		}
	}
	return summary, nil
}

func (s *Service) backupPhoto(image backup.ImageFile, sourcePath, destPath string, duplicatePaths map[string]struct{}) (backup.BackupResult, error) {
	result := backup.BackupResult{OriginalName: image.Name, OriginalPath: image.Path, FileSize: image.Size}
	info, err := os.Stat(image.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return result, fmt.Errorf("ไม่พบไฟล์ต้นทางระหว่าง Backup: %s", image.Name)
		}
		return result, fmt.Errorf("เปิดไฟล์ต้นทางไม่ได้ (%s): %w", image.Name, err)
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
			// รูปเดิมไม่ต้องคัดลอกซ้ำ แต่เติม Hash จากไฟล์ปลายทางได้
			if previous.ContentHash == nil {
				hash, hashErr := analysis.SHA256File(previous.BackupPath)
				if hashErr != nil {
					return result, hashErr
				}
				updateStore, updateCancel := s.operation()
				hashErr = updateStore.UpdatePhotoContentHash(previous.ID, &hash)
				updateCancel()
				if hashErr != nil {
					return result, hashErr
				}
				previous.ContentHash = &hash
			}
			s.enqueueAIJob(previous.ID, previous.ContentHash)
			result.BackupPath = previous.BackupPath
			result.Message = "ใช้ไฟล์ปลายทางเดิม ตรวจสอบแล้วตรงกัน และลบต้นฉบับสำเร็จ"
			if err := s.finishMove(image.Path, result.BackupPath, info); err != nil {
				return result, err
			}
			result.Success, result.Skipped = true, true
			return result, nil
		}
	}

	if err := analysis.ValidateImage(image.Path); err != nil {
		return result, err
	}
	relative, err := filepath.Rel(sourcePath, image.Path)
	if err != nil {
		return result, err
	}
	copyDir := filepath.Join(destPath, filepath.Dir(relative))
	startNumber := 0
	if previous == nil {
		expectedPath := filepath.Join(copyDir, image.Name)
		if _, exists := duplicatePaths[filepath.Clean(expectedPath)]; exists {
			startNumber = 1
		}
	}
	if startNumber == 0 {
		result, err = backup.CopyImage(image, copyDir)
	} else {
		result, err = backup.CopyImageFromNumber(image, copyDir, startNumber)
	}
	if err != nil {
		return result, err
	}
	// Hash หลัง CopyImage ตรวจ byte เทียบกับต้นฉบับแล้ว จึงใช้เป็น identity ของเนื้อหาได้
	contentHash, err := analysis.SHA256File(result.BackupPath)
	if err != nil {
		_ = os.Remove(result.BackupPath)
		result.BackupPath = ""
		return result, fmt.Errorf("คำนวณ ContentHash ของไฟล์ปลายทางไม่ได้: %w", err)
	}
	photo := &model.Photo{
		OriginalName:     image.Name,
		OriginalPath:     image.Path,
		BackupPath:       result.BackupPath,
		FileSize:         image.Size,
		SourceModifiedAt: info.ModTime(),
		BackedUpAt:       time.Now(),
		ContentHash:      &contentHash,
	}
	// ให้ขั้นตอนหนัก ๆ ทำพร้อมกันได้ แต่ให้ SQLite เขียนทีละ transaction
	backupDatabaseLock.Lock()
	store, cancel = s.operation()
	err = store.SaveBackup(photo)
	cancel()
	backupDatabaseLock.Unlock()
	if err != nil {
		// หาก DB บันทึกไม่ได้ ลบเฉพาะสำเนาที่เพิ่งสร้าง และเก็บต้นฉบับไว้
		_ = os.Remove(result.BackupPath)
		result.BackupPath = ""
		return result, fmt.Errorf("บันทึกข้อมูลรูปไม่ได้: %w", err)
	}
	// AI เป็นงานเสริม: ถ้าสร้าง Job ไม่สำเร็จ Backup ยังถือว่าสำเร็จ
	s.enqueueAIJob(photo.ID, photo.ContentHash)
	if err := s.finishMove(image.Path, result.BackupPath, info); err != nil {
		return result, err
	}
	return result, nil
}

func (s *Service) backupPathsInDestination(destPath string) (map[string]struct{}, error) {
	store, cancel := s.operation()
	paths, err := store.BackupPathsInDestination(destPath)
	cancel()
	if err != nil {
		return nil, err
	}
	duplicates := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		duplicates[filepath.Clean(path)] = struct{}{}
	}
	return duplicates, nil
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
		return fmt.Errorf("ไฟล์ต้นทางหายไประหว่าง Backup จึงไม่ถือว่าสำเร็จ: %s", filepath.Base(path))
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
