package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
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
	Retrying   int64 `json:"retrying"`
	Completed  int64 `json:"completed"`
	Failed     int64 `json:"failed"`
}

const (
	analysisBatchSize             = 5
	defaultAnalysisRetries        = 3
	defaultAnalysisRequestTimeout = 2 * time.Minute
	defaultAnalysisJobTimeout     = 15 * time.Minute
	defaultAnalysisRetryDelay     = time.Second
)

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
	for len(ids) < analysisBatchSize {
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

	inputs := make([]analysis.ImageInput, 0, len(photoIDs))
	hashes := make(map[uint]string, len(photoIDs))
	for _, photoID := range photoIDs {
		if s.analysisCtx.Err() != nil {
			return
		}
		store, cancel := s.operation()
		photo, err := store.GetPhoto(photoID)
		cancel()
		if err != nil {
			s.markAnalysisFailed(photoID, fmt.Errorf("อ่านข้อมูลรูปจากฐานข้อมูลไม่สำเร็จ: %w", err))
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
			s.markAnalysisFailed(photoID, fmt.Errorf("อัปเดตสถานะรูปในฐานข้อมูลไม่สำเร็จ: %w", err))
			continue
		}
		s.emitAnalysis(AnalysisProgress{PhotoID: photoID, Status: model.AnalysisProcessing})
		inputs = append(inputs, analysis.ImageInput{ID: photoID, Path: photo.BackupPath})
		hashes[photoID] = contentHash
	}
	if len(inputs) == 0 {
		return
	}

	jobTimeout := time.Duration(analysisEnvInt("AI_ANALYSIS_JOB_TIMEOUT_SECONDS", int(defaultAnalysisJobTimeout/time.Second))) * time.Second
	if jobTimeout <= 0 {
		jobTimeout = defaultAnalysisJobTimeout
	}
	ctx, timeoutCancel := context.WithTimeout(s.analysisCtx, jobTimeout)
	defer timeoutCancel()
	batchRetry := s.analysisRetryNotifier(inputs)
	if batchAnalyzer, ok := s.analyzer.(BatchPhotoAnalyzer); ok && len(inputs) > 1 {
		results, err := s.analyzeBatchWithRetry(analysis.WithRetryNotifier(ctx, batchRetry), batchAnalyzer, inputs)
		if err == nil {
			for _, input := range inputs {
				s.savePhotoAnalysis(input.ID, results[input.ID], hashes[input.ID])
			}
			return
		}
		if ctx.Err() == nil {
			batchRetry()
		}
	}

	for _, input := range inputs {
		retry := s.analysisRetryNotifier([]analysis.ImageInput{input})
		result, err := s.analyzeImageWithRetry(analysis.WithRetryNotifier(ctx, retry), input.Path)
		if err != nil {
			s.markAnalysisFailed(input.ID, err)
			continue
		}
		s.savePhotoAnalysis(input.ID, result, hashes[input.ID])
	}
}

func (s *Service) analysisRetryNotifier(inputs []analysis.ImageInput) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			for _, input := range inputs {
				if err := s.setAnalysisStatus(input.ID, model.AnalysisRetrying, ""); err == nil {
					s.emitAnalysis(AnalysisProgress{PhotoID: input.ID, Status: model.AnalysisRetrying})
				}
			}
		})
	}
}

func (s *Service) analyzeBatchWithRetry(ctx context.Context, analyzer BatchPhotoAnalyzer, inputs []analysis.ImageInput) (map[uint]analysis.Result, error) {
	var lastErr error
	maxRetries := analysisEnvInt("AI_MAX_RETRIES", defaultAnalysisRetries)
	if maxRetries < 0 {
		maxRetries = 0
	}
	if maxRetries > 6 {
		maxRetries = 6
	}
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			analysis.NotifyRetry(ctx)
			if !waitForAnalysisRetry(ctx, attempt) {
				return nil, ctx.Err()
			}
		}
		attemptCtx, cancel := analysisAttemptContext(ctx)
		results, err := analyzer.AnalyzeImages(attemptCtx, inputs)
		cancel()
		if err == nil {
			err = validateBatchResults(inputs, results)
		}
		if err == nil {
			return results, nil
		}
		lastErr = err
		if !canRetryAnalysis(ctx, err) {
			break
		}
	}
	if lastErr == nil {
		lastErr = errors.New("การวิเคราะห์แบบชุดไม่สำเร็จ")
	}
	return nil, lastErr
}

func (s *Service) analyzeImageWithRetry(ctx context.Context, path string) (analysis.Result, error) {
	var lastErr error
	maxRetries := analysisEnvInt("AI_MAX_RETRIES", defaultAnalysisRetries)
	if maxRetries < 0 {
		maxRetries = 0
	}
	if maxRetries > 6 {
		maxRetries = 6
	}
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			analysis.NotifyRetry(ctx)
			if !waitForAnalysisRetry(ctx, attempt) {
				return analysis.Result{}, ctx.Err()
			}
		}
		attemptCtx, cancel := analysisAttemptContext(ctx)
		result, err := s.analyzer.AnalyzeImage(attemptCtx, path)
		cancel()
		if err == nil {
			err = validateAnalysisResult(result)
		}
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !canRetryAnalysis(ctx, err) {
			break
		}
	}
	if lastErr == nil {
		lastErr = errors.New("การวิเคราะห์รูปไม่สำเร็จ")
	}
	return analysis.Result{}, lastErr
}

func validateBatchResults(inputs []analysis.ImageInput, results map[uint]analysis.Result) error {
	if len(results) != len(inputs) {
		return fmt.Errorf("AI ส่งผลลัพธ์ไม่ครบ: ต้องการ %d รูป แต่ได้ %d รูป", len(inputs), len(results))
	}
	for _, input := range inputs {
		result, exists := results[input.ID]
		if !exists {
			return fmt.Errorf("AI ไม่ส่งผลลัพธ์ของรูป ID %d", input.ID)
		}
		if err := validateAnalysisResult(result); err != nil {
			return fmt.Errorf("ผลของรูป ID %d ไม่สมบูรณ์: %w", input.ID, err)
		}
	}
	return nil
}

func validateAnalysisResult(result analysis.Result) error {
	if strings.TrimSpace(result.Description) == "" {
		return errors.New("AI ไม่ส่งคำอธิบายภาพ")
	}
	for _, tag := range result.Tags {
		if strings.TrimSpace(tag) != "" {
			return nil
		}
	}
	return errors.New("AI ไม่ส่ง Tag ของภาพ")
}

func analysisAttemptContext(parent context.Context) (context.Context, context.CancelFunc) {
	seconds := analysisEnvInt("AI_ANALYSIS_REQUEST_TIMEOUT_SECONDS", int(defaultAnalysisRequestTimeout/time.Second))
	if seconds <= 0 {
		seconds = int(defaultAnalysisRequestTimeout / time.Second)
	}
	return context.WithTimeout(parent, time.Duration(seconds)*time.Second)
}

func waitForAnalysisRetry(ctx context.Context, attempt int) bool {
	baseMillis := analysisEnvInt("AI_RETRY_BASE_DELAY_MS", int(defaultAnalysisRetryDelay/time.Millisecond))
	if baseMillis < 0 {
		baseMillis = 0
	}
	if attempt > 6 {
		attempt = 6
	}
	delay := time.Duration(baseMillis) * time.Millisecond * time.Duration(1<<uint(attempt-1))
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func canRetryAnalysis(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if ctx.Err() != nil {
		return false
	}
	return !errors.Is(err, context.Canceled)
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
	if s.analysisCtx.Err() != nil {
		return
	}
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
		Retrying:   counts.Retrying,
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
			s.emitAnalysis(AnalysisProgress{PhotoID: id, Status: model.AnalysisPending})
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
