package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"

	"photo-backup/analysis"
	"photo-backup/model"
)

type PhotoAnalyzer interface {
	AnalyzeImage(context.Context, string) (analysis.Result, error)
}

type BatchPhotoAnalyzer interface {
	AnalyzeImages(context.Context, []analysis.ImageInput) (map[uint]analysis.Result, error)
}

type AnalysisProgress struct {
	PhotoID uint   `json:"photoId"`
	Status  string `json:"status"`
	Error   string `json:"error"`
}

type AnalysisSummary struct {
	Enabled    bool  `json:"enabled"`
	Pending    int64 `json:"pending"`
	Processing int64 `json:"processing"`
	Completed  int64 `json:"completed"`
	Failed     int64 `json:"failed"`
}

func (s *Service) SetAnalysisNotifier(notifier func(AnalysisProgress)) {
	s.analysisMu.Lock()
	defer s.analysisMu.Unlock()
	s.notifyAnalysis = notifier
}

func (s *Service) startAnalysisWorkers(count int) {
	for i := 0; i < count; i++ {
		s.analysisWG.Add(1)
		go func() {
			defer s.analysisWG.Done()
			for {
				select {
				case <-s.analysisCtx.Done():
					return
				case photoID := <-s.analysisJobs:
					s.processPhotoAnalysisBatch(s.collectAnalysisBatch(photoID))
				}
			}
		}()
	}
}

func (s *Service) collectAnalysisBatch(firstID uint) []uint {
	ids := []uint{firstID}
	timer := time.NewTimer(25 * time.Millisecond)
	defer timer.Stop()
	for len(ids) < 5 {
		select {
		case photoID := <-s.analysisJobs:
			ids = append(ids, photoID)
		case <-timer.C:
			return ids
		case <-s.analysisCtx.Done():
			return ids
		}
	}
	return ids
}

func (s *Service) resumePendingAnalysis() {
	store, cancel := s.operation()
	ids, err := store.PendingAnalysisPhotoIDs()
	cancel()
	if err != nil {
		return
	}
	for _, id := range ids {
		s.enqueuePhotoAnalysis(id)
	}
}

func (s *Service) enqueuePhotoAnalysis(photoID uint) {
	if s == nil || s.analyzer == nil || photoID == 0 {
		return
	}
	s.analysisMu.Lock()
	if s.analysisClosed {
		s.analysisMu.Unlock()
		return
	}
	if _, exists := s.analysisQueued[photoID]; exists {
		s.analysisMu.Unlock()
		return
	}
	s.analysisQueued[photoID] = struct{}{}
	s.analysisMu.Unlock()

	select {
	case s.analysisJobs <- photoID:
	case <-s.analysisCtx.Done():
		s.removeQueuedPhoto(photoID)
	}
}

func (s *Service) removeQueuedPhoto(photoID uint) {
	s.analysisMu.Lock()
	delete(s.analysisQueued, photoID)
	s.analysisMu.Unlock()
}

func (s *Service) processPhotoAnalysisBatch(photoIDs []uint) {
	for _, photoID := range photoIDs {
		defer s.removeQueuedPhoto(photoID)
	}
	store, cancel := s.operation()
	cancel()

	inputs := make([]analysis.ImageInput, 0, len(photoIDs))
	hashes := make(map[uint]string, len(photoIDs))
	for _, photoID := range photoIDs {
		store, cancel = s.operation()
		photo, err := store.GetPhoto(photoID)
		cancel()
		if err != nil {
			continue
		}
		contentHash, err := hashFile(photo.BackupPath)
		if err != nil {
			s.markAnalysisFailed(photoID, fmt.Errorf("อ่านไฟล์ปลายทางไม่สำเร็จ: %w", err))
			continue
		}
		if photo.AnalysisStatus == model.AnalysisCompleted && photo.ContentHash == contentHash {
			continue
		}
		if err := s.setAnalysisStatus(photoID, model.AnalysisProcessing, ""); err != nil {
			continue
		}
		s.emitAnalysis(AnalysisProgress{PhotoID: photoID, Status: model.AnalysisProcessing})
		inputs = append(inputs, analysis.ImageInput{ID: photoID, Path: photo.BackupPath})
		hashes[photoID] = contentHash
	}
	if len(inputs) == 0 {
		return
	}

	ctx, timeoutCancel := context.WithTimeout(s.analysisCtx, 5*time.Minute)
	defer timeoutCancel()
	if batchAnalyzer, ok := s.analyzer.(BatchPhotoAnalyzer); ok && len(inputs) > 1 {
		results, err := batchAnalyzer.AnalyzeImages(ctx, inputs)
		if err == nil {
			for _, input := range inputs {
				result, exists := results[input.ID]
				if !exists {
					s.markAnalysisFailed(input.ID, fmt.Errorf("Gemini ไม่ส่งผลลัพธ์ของรูป ID %d", input.ID))
					continue
				}
				s.savePhotoAnalysis(input.ID, result, hashes[input.ID])
			}
			return
		}
	}

	for _, input := range inputs {
		result, err := s.analyzer.AnalyzeImage(ctx, input.Path)
		if err != nil {
			s.markAnalysisFailed(input.ID, err)
			continue
		}
		s.savePhotoAnalysis(input.ID, result, hashes[input.ID])
	}
}

func (s *Service) savePhotoAnalysis(photoID uint, result analysis.Result, contentHash string) {
	store, cancel := s.operation()
	err := store.SavePhotoAnalysis(photoID, result.Description, result.Tags, contentHash, time.Now())
	cancel()
	if err != nil {
		s.markAnalysisFailed(photoID, fmt.Errorf("บันทึกผลวิเคราะห์ไม่ได้: %w", err))
		return
	}
	s.emitAnalysis(AnalysisProgress{PhotoID: photoID, Status: model.AnalysisCompleted})
}

func (s *Service) setAnalysisStatus(photoID uint, status, message string) error {
	store, cancel := s.operation()
	defer cancel()
	return store.SetPhotoAnalysisStatus(photoID, status, message)
}

func (s *Service) markAnalysisFailed(photoID uint, err error) {
	message := "การวิเคราะห์ไม่สำเร็จ"
	if err != nil {
		message = err.Error()
	}
	if updateErr := s.setAnalysisStatus(photoID, model.AnalysisFailed, message); updateErr != nil {
		message = updateErr.Error()
	}
	s.emitAnalysis(AnalysisProgress{PhotoID: photoID, Status: model.AnalysisFailed, Error: message})
}

func (s *Service) emitAnalysis(progress AnalysisProgress) {
	s.analysisMu.Lock()
	notifier := s.notifyAnalysis
	s.analysisMu.Unlock()
	if notifier != nil {
		notifier(progress)
	}
}

func (s *Service) AnalysisStatus() (AnalysisSummary, error) {
	store, cancel := s.operation()
	counts, err := store.AnalysisCounts()
	cancel()
	if err != nil {
		return AnalysisSummary{}, err
	}
	return AnalysisSummary{
		Enabled:    s.analyzer != nil,
		Pending:    counts.Pending,
		Processing: counts.Processing,
		Completed:  counts.Completed,
		Failed:     counts.Failed,
	}, nil
}

func (s *Service) RetryFailedAnalysis() int {
	if s.analyzer == nil {
		return 0
	}
	store, cancel := s.operation()
	ids, err := store.FailedAnalysisPhotoIDs()
	cancel()
	if err != nil {
		return 0
	}
	for _, id := range ids {
		if err := s.setAnalysisStatus(id, model.AnalysisPending, ""); err == nil {
			s.enqueuePhotoAnalysis(id)
		}
	}
	return len(ids)
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
