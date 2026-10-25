package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"photo-backup/analysis"
	"photo-backup/model"
)

const (
	aiJobQueued     = "queued"
	aiJobProcessing = "processing"
	aiJobCompleted  = "completed"
	aiJobFailed     = "failed"
	aiJobRetryWait  = "retry_wait"
)

// AIQueueStatus เป็นข้อมูลสรุปที่ UI ใช้แสดงความคืบหน้าโดยไม่ต้องอ่านตารางเอง
type AIQueueStatus struct {
	Queued     int64 `json:"queued"`
	Processing int64 `json:"processing"`
	Completed  int64 `json:"completed"`
	Failed     int64 `json:"failed"`
	RetryWait  int64 `json:"retryWait"`
}

type AIProviderStatus struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	State    string `json:"state"`
	Message  string `json:"message"`
}

var aiRateGate = struct {
	sync.Mutex
	next map[string]time.Time
}{next: make(map[string]time.Time)}

func (s *Service) enqueueAIJob(photoID uint, contentHash *string) {
	if s.analyzer == nil || contentHash == nil || strings.TrimSpace(*contentHash) == "" {
		return
	}
	provider, modelName := analysis.ConfiguredProviderModel()
	job := &model.AIJob{
		PhotoID: photoID, ContentHash: *contentHash,
		AnalysisProfile: analysis.DefaultAnalysisProfile,
		PromptVersion:   analysis.DefaultPromptVersion,
		Provider:        provider, Model: modelName, Status: aiJobQueued,
	}
	store, cancel := s.operation()
	backupDatabaseLock.Lock()
	_, _ = store.CreateAIJobIfMissing(job)
	backupDatabaseLock.Unlock()
	cancel()
}

func (s *Service) runAIQueue(ctx context.Context) {
	store, cancel := s.operation()
	backupDatabaseLock.Lock()
	_ = store.ResetProcessingAIJobs()
	backupDatabaseLock.Unlock()
	cancel()
	s.reconcileAIJobs()

	// Two workers reduce total time on machines with multiple CPU cores while
	// remaining conservative for the local vision model. Users can set
	// AI_MAX_CONCURRENCY=1 if RAM/CPU is limited.
	workerCount := envInt("AI_MAX_CONCURRENCY", 2)
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > 4 {
		workerCount = 4
	}
	var workers sync.WaitGroup
	for index := 0; index < workerCount; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			s.aiWorker(ctx)
		}()
	}
	workers.Wait()
}

func (s *Service) reconcileAIJobs() {
	if s.analyzer == nil {
		return
	}
	store, cancel := s.operation()
	photos, err := store.ListPhotos()
	cancel()
	if err != nil {
		return
	}
	for _, photo := range photos {
		if photo.ContentHash == nil || strings.TrimSpace(*photo.ContentHash) == "" || strings.TrimSpace(photo.Description) != "" {
			continue
		}
		s.enqueueAIJob(photo.ID, photo.ContentHash)
	}
}

func (s *Service) aiWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		store, cancel := s.operation()
		backupDatabaseLock.Lock()
		job, err := store.ClaimNextAIJob(time.Now())
		backupDatabaseLock.Unlock()
		cancel()
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
			time.Sleep(250 * time.Millisecond)
			continue
		}
		if job == nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
			}
			continue
		}
		s.processAIJob(ctx, job)
	}
}

func (s *Service) processAIJob(ctx context.Context, job *model.AIJob) {
	finish := func(status, message string) {
		now := time.Now()
		job.Status = status
		job.LastError = sanitizeAIError(message)
		job.FinishedAt = &now
		job.NextRetryAt = nil
		store, cancel := s.operation()
		backupDatabaseLock.Lock()
		_ = store.UpdateAIJob(job)
		backupDatabaseLock.Unlock()
		cancel()
		s.emitAIProgress()
	}
	retry := func(message string, cause error) bool {
		if !isRetryableAIError(cause) || job.AttemptCount > envInt("AI_MAX_RETRIES", 3) {
			return false
		}
		delay := retryDelay(cause, job.AttemptCount)
		next := time.Now().Add(delay)
		job.Status = aiJobRetryWait
		job.LastError = sanitizeAIError(message)
		job.NextRetryAt = &next
		job.FinishedAt = nil
		store, cancel := s.operation()
		backupDatabaseLock.Lock()
		_ = store.UpdateAIJob(job)
		backupDatabaseLock.Unlock()
		cancel()
		s.emitAIProgress()
		return true
	}

	select {
	case <-ctx.Done():
		return
	default:
	}
	store, cancel := s.operation()
	photo, err := store.FindPhoto(job.PhotoID)
	cancel()
	if err != nil {
		finish(aiJobFailed, fmt.Sprintf("หารูปสำหรับ Job ไม่พบ: %v", err))
		return
	}
	if photo.ContentHash == nil || *photo.ContentHash != job.ContentHash {
		finish(aiJobFailed, "ContentHash ของ Photo ไม่ตรงกับ Job")
		return
	}
	info, err := os.Stat(photo.BackupPath)
	if err != nil || !info.Mode().IsRegular() {
		finish(aiJobFailed, "ไม่พบไฟล์ Backup สำหรับวิเคราะห์; ไม่แตะไฟล์ต้นฉบับ")
		return
	}
	hash, err := analysis.SHA256File(photo.BackupPath)
	if err != nil {
		finish(aiJobFailed, fmt.Sprintf("อ่านไฟล์ Backup ไม่ได้: %v", err))
		return
	}
	if hash != job.ContentHash {
		finish(aiJobFailed, "ไฟล์ Backup เปลี่ยนเนื้อหา จึงไม่ใช้ผลวิเคราะห์เดิม")
		return
	}

	store, cancel = s.operation()
	cached, err := store.FindAIResult(job.ContentHash, job.AnalysisProfile, job.PromptVersion)
	cancel()
	if err != nil {
		finish(aiJobFailed, fmt.Sprintf("อ่าน AI Cache ไม่ได้: %v", err))
		return
	}
	if cached != nil {
		var tags []string
		if json.Unmarshal([]byte(cached.TagsJSON), &tags) != nil {
			finish(aiJobFailed, "AI Cache มี TagsJSON ไม่ถูกต้อง")
			return
		}
		// ใช้ composite cache key เป็นตัวชน ไม่ใช้ ID เดิมที่อ่านจาก DB
		cachedCopy := *cached
		cachedCopy.ID = 0
		if err := s.saveCachedResult(&cachedCopy, tags); err != nil {
			finish(aiJobFailed, fmt.Sprintf("ใช้ AI Cache ไม่ได้: %v", err))
			return
		}
		finish(aiJobCompleted, "")
		return
	}

	provider, modelName := analysis.ConfiguredProviderModel()
	limit := envInt("AI_DAILY_LIMIT", 0)
	store, cancel = s.operation()
	backupDatabaseLock.Lock()
	allowed, quotaErr := store.ConsumeAIRequest(provider, modelName, limit)
	backupDatabaseLock.Unlock()
	cancel()
	if quotaErr != nil {
		finish(aiJobFailed, fmt.Sprintf("อ่านโควตา AI ไม่ได้: %v", quotaErr))
		return
	}
	if !allowed {
		next := nextLocalDay()
		job.Status = aiJobRetryWait
		job.LastError = "ถึง AI_DAILY_LIMIT แล้ว จึงพักงานไว้จนถึงวันถัดไป"
		job.NextRetryAt = &next
		job.FinishedAt = nil
		store, cancel = s.operation()
		backupDatabaseLock.Lock()
		_ = store.UpdateAIJob(job)
		backupDatabaseLock.Unlock()
		cancel()
		s.emitAIProgress()
		return
	}
	if err := waitForProvider(ctx, provider+":"+modelName); err != nil {
		return
	}
	result, err := s.analyzer.Analyze(photo.BackupPath)
	if err != nil {
		if retry(err.Error(), err) {
			return
		}
		finish(aiJobFailed, err.Error())
		return
	}
	tagsJSON, err := json.Marshal(result.Tags)
	if err != nil {
		finish(aiJobFailed, fmt.Sprintf("แปลง Tags เป็น JSON ไม่ได้: %v", err))
		return
	}
	cached = &model.AIResult{
		ContentHash: job.ContentHash, AnalysisProfile: job.AnalysisProfile,
		Provider: provider, Model: modelName, PromptVersion: job.PromptVersion,
		Description: result.Description, TagsJSON: string(tagsJSON),
	}
	if err := s.saveCachedResult(cached, result.Tags); err != nil {
		finish(aiJobFailed, fmt.Sprintf("บันทึกผลวิเคราะห์ไม่ได้: %v", err))
		return
	}
	finish(aiJobCompleted, "")
}

func (s *Service) saveCachedResult(result *model.AIResult, tags []string) error {
	backupDatabaseLock.Lock()
	defer backupDatabaseLock.Unlock()
	store, cancel := s.operation()
	defer cancel()
	return store.SaveAIResultAndApply(result, tags)
}

func (s *Service) emitAIProgress() {
	if s.aiProgress == nil {
		return
	}
	status, err := s.GetAIQueueStatus()
	if err == nil {
		s.aiProgress(status)
	}
}

func (s *Service) GetAIQueueStatus() (AIQueueStatus, error) {
	store, cancel := s.operation()
	defer cancel()
	counts, err := store.AIJobCounts()
	if err != nil {
		return AIQueueStatus{}, err
	}
	return AIQueueStatus{
		Queued: counts[aiJobQueued], Processing: counts[aiJobProcessing],
		Completed: counts[aiJobCompleted], Failed: counts[aiJobFailed], RetryWait: counts[aiJobRetryWait],
	}, nil
}

func (s *Service) GetAIProviderStatus(ctx context.Context) AIProviderStatus {
	provider, modelName := analysis.ConfiguredProviderModel()
	if s.analyzer == nil {
		return AIProviderStatus{Provider: provider, Model: modelName, State: "disabled", Message: "AI ถูกปิดไว้ตามค่าเริ่มต้นที่ปลอดภัย"}
	}
	if multi, ok := s.analyzer.(*analysis.MultiAnalyzer); ok {
		return AIProviderStatus{Provider: provider, Model: modelName, State: "configured", Message: fmt.Sprintf("พร้อมใช้งานหลาย Provider (%d ตัว)", multi.ProviderCount())}
	}
	if combined, ok := s.analyzer.(*analysis.CombinedAnalyzer); ok {
		state, message := combined.LocalStatus(ctx)
		return AIProviderStatus{Provider: provider, Model: modelName, State: state, Message: message}
	}
	return AIProviderStatus{Provider: provider, Model: modelName, State: "configured"}
}

func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil {
		return fallback
	}
	return value
}

func isRetryableAIError(err error) bool {
	if err == nil {
		return false
	}
	var providerErr *analysis.ProviderError
	if errors.As(err, &providerErr) {
		return providerErr.StatusCode == 429 || providerErr.StatusCode >= 500
	}
	var networkErr net.Error
	return errors.As(err, &networkErr)
}

func retryDelay(err error, attempt int) time.Duration {
	var providerErr *analysis.ProviderError
	if errors.As(err, &providerErr) && providerErr.RetryAfter > 0 {
		return providerErr.RetryAfter
	}
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 6 {
		attempt = 6
	}
	return time.Duration(1<<uint(attempt-1))*time.Second + time.Duration((attempt*137)%500)*time.Millisecond
}

func waitForProvider(ctx context.Context, key string) error {
	interval := envInt("AI_MIN_INTERVAL_MS", 0)
	if interval <= 0 {
		return nil
	}
	wait := time.Duration(interval) * time.Millisecond
	aiRateGate.Lock()
	now := time.Now()
	start := now
	if next, ok := aiRateGate.next[key]; ok && next.After(start) {
		start = next
	}
	aiRateGate.next[key] = start.Add(wait)
	aiRateGate.Unlock()
	if delay := time.Until(start); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

func nextLocalDay() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
}

func sanitizeAIError(message string) string {
	for _, envName := range []string{"GEMINI_API_KEY", "GEMINI_API_KEYS", "OPENAI_API_KEY", "OPENAI_API_KEYS"} {
		for _, key := range strings.FieldsFunc(os.Getenv(envName), func(r rune) bool { return r == ',' || r == ';' || r == '\n' || r == '\r' }) {
			key = strings.TrimSpace(key)
			if key != "" {
				message = strings.ReplaceAll(message, key, "[REDACTED]")
			}
		}
	}
	return message
}
