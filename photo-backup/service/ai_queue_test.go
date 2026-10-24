package service_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"photo-backup/analysis"
	"photo-backup/service"
)

type countingAnalyzer struct {
	mu    sync.Mutex
	calls int
}

type slowAnalyzer struct{}

func (slowAnalyzer) Analyze(string) (analysis.AIResult, error) {
	time.Sleep(5 * time.Millisecond)
	return analysis.AIResult{Description: "concurrent test", Tags: []string{"concurrent"}}, nil
}

func (a *countingAnalyzer) Analyze(string) (analysis.AIResult, error) {
	a.mu.Lock()
	a.calls++
	a.mu.Unlock()
	return analysis.AIResult{Description: "ภาพจาก Fake Analyzer", Tags: []string{"ทดสอบ", "รูปภาพ"}}, nil
}

func (a *countingAnalyzer) Calls() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calls
}

func waitForAIQueue(t *testing.T, s *service.Service) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status, err := s.GetAIQueueStatus()
		if err == nil && status.Queued == 0 && status.Processing == 0 && status.RetryWait == 0 && status.Completed > 0 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	status, _ := s.GetAIQueueStatus()
	t.Fatalf("AI queue did not complete: %+v", status)
}

func TestBackupCreatesPersistentAIJobAndCacheHitAvoidsSecondAnalyzerCall(t *testing.T) {
	s := newService(t)
	s.SetContext(context.Background())
	analyzer := &countingAnalyzer{}
	s.SetImageAnalyzer(analyzer)
	t.Cleanup(s.Shutdown)
	source, destination := t.TempDir(), t.TempDir()
	firstPath := filepath.Join(source, "first.png")
	writeSamplePhoto(t, firstPath)
	if summary, err := s.StartBackup(source, destination, nil); err != nil || summary.Success != 1 {
		t.Fatalf("first backup: %+v, %v", summary, err)
	}
	waitForAIQueue(t, s)
	if analyzer.Calls() != 1 {
		t.Fatalf("analyzer calls after first photo = %d, want 1", analyzer.Calls())
	}
	secondPath := filepath.Join(source, "second.png")
	data, err := os.ReadFile(filepath.Join(destination, "first.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if summary, err := s.StartBackup(source, destination, nil); err != nil || summary.Success != 1 {
		t.Fatalf("second backup: %+v, %v", summary, err)
	}
	waitForAIQueue(t, s)
	if analyzer.Calls() != 1 {
		t.Fatalf("cache miss unexpectedly called analyzer again: %d", analyzer.Calls())
	}
}

func TestBackupAndAIWorkerConcurrentWritesDoNotLockSQLite(t *testing.T) {
	t.Setenv("AI_MAX_CONCURRENCY", "2")
	s := newService(t)
	s.SetContext(context.Background())
	s.SetImageAnalyzer(slowAnalyzer{})
	t.Cleanup(s.Shutdown)
	source, destination := t.TempDir(), t.TempDir()
	for index := 0; index < 20; index++ {
		writeSamplePhoto(t, filepath.Join(source, fmt.Sprintf("photo-%02d.png", index)))
	}
	summary, err := s.StartBackup(source, destination, nil)
	if err != nil || summary.Success != 20 {
		t.Fatalf("concurrent backup: success=%d failed=%d err=%v", summary.Success, summary.Failed, err)
	}
	waitForAIQueue(t, s)
	status, err := s.GetAIQueueStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Failed != 0 || status.Completed != 20 {
		t.Fatalf("concurrent backup/AI writes: %+v (a database lock would appear as a failed job)", status)
	}
}
