package service

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"photo-backup/backup"
	"photo-backup/model"
)

// ป้องกันกดสำรองซ้อนกันระหว่างที่ยังทำงานกับไฟล์ชุดเดิม
var backupLock sync.Mutex

type preparedBackup struct {
	image      backup.ImageFile
	result     backup.BackupResult
	sourceInfo os.FileInfo
	photo      *model.Photo
	analysisID uint
	canFinish  bool
}

type backupJob struct {
	index int
	image backup.ImageFile
}

type backupJobResult struct {
	index    int
	prepared preparedBackup
	err      error
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

// StartBackup: ตรวจโฟลเดอร์ → เตรียมปลายทาง → บันทึก DB → ลบต้นทาง
// การวิเคราะห์ AI จะถูกส่งเข้าคิวหลังจากขั้นตอน Backup จบแล้ว
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

	summary := backup.BackupSummary{Total: len(images), Results: make([]backup.BackupResult, len(images))}
	for completed := range s.prepareBackups(images, sourcePath, destPath, duplicatePaths) {
		result := completed.prepared.result
		if completed.err == nil {
			result, completed.err = s.commitPreparedBackup(completed.prepared)
		}
		if completed.err != nil {
			result.Success = false
			result.Skipped = false
			result.Error = completed.err.Error()
			summary.Failed++
		} else if result.Skipped {
			summary.Skipped++
		} else {
			summary.Success++
		}
		summary.Results[completed.index] = result
		if onProgress != nil {
			onProgress(summary.Success+summary.Skipped+summary.Failed, len(images), result.OriginalName)
		}
	}
	return summary, nil
}

// prepareBackups performs the expensive file work concurrently. SQLite writes
// are intentionally kept out of this stage so that many workers do not fight
// over the same database lock.
func (s *Service) prepareBackups(images []backup.ImageFile, sourcePath, destPath string, duplicatePaths map[string]struct{}) <-chan backupJobResult {
	results := make(chan backupJobResult, len(images))
	jobs := make(chan backupJob)
	workers := backupWorkerCount(len(images))
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				prepared, err := s.prepareBackup(job.image, sourcePath, destPath, duplicatePaths)
				results <- backupJobResult{index: job.index, prepared: prepared, err: err}
			}
		}()
	}

	go func() {
		for index, image := range images {
			jobs <- backupJob{index: index, image: image}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	return results
}

func backupWorkerCount(jobCount int) int {
	if jobCount < 2 {
		return 1
	}
	workers := runtime.NumCPU()
	if workers < 2 {
		workers = 2
	}
	if workers > 4 {
		workers = 4
	}
	if workers > jobCount {
		workers = jobCount
	}
	return workers
}

func (s *Service) prepareBackup(image backup.ImageFile, sourcePath, destPath string, duplicatePaths map[string]struct{}) (preparedBackup, error) {
	prepared := preparedBackup{
		image:  image,
		result: backup.BackupResult{OriginalName: image.Name, OriginalPath: image.Path, FileSize: image.Size},
	}
	info, err := os.Stat(image.Path)
	if err != nil {
		return prepared, err
	}
	image.Size, prepared.result.FileSize = info.Size(), info.Size()
	prepared.sourceInfo = info
	if err := backup.ValidateImage(image.Path); err != nil {
		return prepared, err
	}
	relative, err := filepath.Rel(sourcePath, image.Path)
	if err != nil {
		return prepared, err
	}
	expectedPath := filepath.Clean(filepath.Join(destPath, relative))
	store, cancel := s.operation()
	previous, err := store.FindBackup(image.Path, destPath)
	cancel()
	if err != nil {
		return prepared, err
	}
	if alreadyBackedUp(previous, info, destPath) {
		equal, err := backup.EqualFiles(image.Path, previous.BackupPath)
		if err != nil {
			return prepared, err
		}
		if equal {
			prepared.result.BackupPath = previous.BackupPath
			prepared.result.Success, prepared.result.Skipped = true, true
			prepared.result.Message = "ใช้ไฟล์ปลายทางเดิม ตรวจสอบแล้วตรงกัน และเตรียมลบต้นฉบับ"
			prepared.analysisID = previous.ID
			prepared.canFinish = true
			return prepared, nil
		}
	}
	if previous == nil {
		if _, err := os.Lstat(expectedPath); err == nil {
			prepared.result.Success = true
			prepared.result.Skipped = true
			prepared.result.Message = "พบชื่อไฟล์ซ้ำในโฟลเดอร์ปลายทาง จึงไม่ย้ายไฟล์ และเก็บต้นฉบับไว้"
			return prepared, nil
		} else if !os.IsNotExist(err) {
			return prepared, err
		}
		if _, exists := duplicatePaths[expectedPath]; exists {
			prepared.result.Success = true
			prepared.result.Skipped = true
			prepared.result.Message = "พบประวัติชื่อไฟล์ซ้ำในฐานข้อมูล จึงไม่ย้ายไฟล์ และเก็บต้นฉบับไว้"
			return prepared, nil
		}
	}

	prepared.result, err = backup.CopyImage(image, filepath.Join(destPath, filepath.Dir(relative)))
	if err != nil {
		return prepared, err
	}
	prepared.photo = &model.Photo{
		OriginalName:     image.Name,
		OriginalPath:     image.Path,
		BackupPath:       prepared.result.BackupPath,
		FileSize:         image.Size,
		AnalysisStatus:   model.AnalysisPending,
		SourceModifiedAt: info.ModTime(),
		BackedUpAt:       time.Now(),
	}
	prepared.canFinish = true
	return prepared, nil
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

func (s *Service) commitPreparedBackup(prepared preparedBackup) (backup.BackupResult, error) {
	result := prepared.result
	if prepared.photo != nil {
		store, cancel := s.operation()
		err := store.SaveBackup(prepared.photo)
		cancel()
		if err != nil {
			// หาก DB บันทึกไม่ได้ ลบเฉพาะสำเนาที่เพิ่งสร้าง และเก็บต้นฉบับไว้
			_ = os.Remove(result.BackupPath)
			result.BackupPath = ""
			return result, fmt.Errorf("บันทึกข้อมูลรูปไม่ได้: %w", err)
		}
		s.enqueuePhotoAnalysis(prepared.photo.ID)
	} else if prepared.analysisID != 0 {
		s.enqueuePhotoAnalysis(prepared.analysisID)
	}
	if prepared.canFinish {
		if err := s.finishMove(prepared.image.Path, result.BackupPath, prepared.sourceInfo); err != nil {
			return result, err
		}
	}
	result.Success = true
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
