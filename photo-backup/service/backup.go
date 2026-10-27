package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	duplicateNames, err := s.backupNamesInDestination(destPath)
	if err != nil {
		return backup.BackupSummary{}, err
	}
	duplicateSources := duplicateNames.duplicates(images)

	summary := backup.BackupSummary{Total: len(images), Results: make([]backup.BackupResult, len(images))}
	analysisIDs := make([]uint, 0, len(images))
	for completed := range s.prepareBackups(images, sourcePath, destPath, duplicateSources) {
		result := completed.prepared.result
		if completed.err == nil {
			result, completed.err = s.commitPreparedBackup(completed.prepared)
			if completed.prepared.photo != nil && completed.prepared.photo.ID != 0 && result.BackupPath != "" {
				analysisIDs = append(analysisIDs, completed.prepared.photo.ID)
			}
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
	for _, id := range analysisIDs {
		s.enqueuePhotoAnalysis(id)
	}
	return summary, nil
}

// prepareBackups performs the expensive file work concurrently. SQLite writes
// are intentionally kept out of this stage so that many workers do not fight
// over the same database lock.
func (s *Service) prepareBackups(images []backup.ImageFile, sourcePath, destPath string, duplicateSources map[string]string) <-chan backupJobResult {
	results := make(chan backupJobResult, backupWorkerCount(len(images)))
	jobs := make(chan backupJob)
	workers := backupWorkerCount(len(images))
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				prepared, err := s.prepareBackup(job.image, sourcePath, destPath, duplicateSources)
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

func (s *Service) prepareBackup(image backup.ImageFile, sourcePath, destPath string, duplicateSources map[string]string) (preparedBackup, error) {
	prepared := preparedBackup{
		image:  image,
		result: backup.BackupResult{OriginalName: image.Name, OriginalPath: image.Path, FileSize: image.Size},
	}
	relative, err := filepath.Rel(sourcePath, image.Path)
	if err != nil {
		return prepared, err
	}
	if previous, duplicate := duplicateSources[image.Path]; duplicate {
		prepared.result.Success, prepared.result.Skipped = true, true
		prepared.result.Message = fmt.Sprintf("ชื่อไฟล์ซ้ำกับ %s จึงข้ามและเก็บรูปต้นฉบับไว้", previous)
		return prepared, nil
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
	destinationDir := filepath.Join(destPath, filepath.Dir(relative))
	if err := backup.ValidateDestinationPath(destPath, destinationDir); err != nil {
		return prepared, err
	}

	prepared.result, err = backup.CopyImage(image, destinationDir)
	if err != nil {
		if errors.Is(err, backup.ErrDuplicateName) {
			prepared.result.Success, prepared.result.Skipped = true, true
			prepared.result.Message = err.Error()
			return prepared, nil
		}
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
		return fmt.Errorf("ปลายทางและฐานข้อมูลพร้อมแล้ว แต่ลบต้นทางไม่ได้ ต้นฉบับยังอยู่ กรุณาตรวจสอบสิทธิ์และไฟล์ทั้งสองฝั่ง: %w", err)
	}
	return nil
}
