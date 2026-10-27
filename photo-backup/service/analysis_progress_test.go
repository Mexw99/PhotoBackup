package service

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"photo-backup/analysis"
	"photo-backup/database"
	"photo-backup/model"
	"photo-backup/repository"
)

type progressTestAnalyzer struct {
	calls         int
	internalRetry bool
	alwaysFail    bool
}

func (a *progressTestAnalyzer) AnalyzeImage(ctx context.Context, _ string) (analysis.Result, error) {
	a.calls++
	if a.internalRetry {
		analysis.NotifyRetry(ctx)
	} else if a.calls == 1 || a.alwaysFail {
		return analysis.Result{}, fmt.Errorf("simulated failure")
	}
	return analysis.Result{Description: "ผลทดสอบ", Tags: []string{"ทดสอบ"}}, nil
}

func TestAnalysisProgressIncludesPersistedRetryState(t *testing.T) {
	for _, test := range []struct {
		name     string
		analyzer PhotoAnalyzer
		count    int
		retries  string
		failed   bool
	}{
		{"batch retry", &retryBatchPhotoAnalyzer{failedBatches: 1}, 5, "1", false},
		{"batch fallback", &incompleteBatchPhotoAnalyzer{}, 5, "0", false},
		{"single retry", &progressTestAnalyzer{}, 1, "1", false},
		{"HTTP internal retry", &progressTestAnalyzer{internalRetry: true}, 1, "1", false},
		{"retry exhausted", &progressTestAnalyzer{alwaysFail: true}, 1, "1", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("AI_MAX_RETRIES", test.retries)
			t.Setenv("AI_RETRY_BASE_DELAY_MS", "0")
			db, err := database.OpenAt(filepath.Join(t.TempDir(), "progress.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			store := repository.New(db.GORM())
			s := NewWithAnalyzer(store, nil)
			defer s.Close()
			s.analyzer = test.analyzer
			ids := createAnalysisTestPhotos(t, store, t.TempDir(), test.count)
			maxRetrying := int64(0)
			s.SetAnalysisNotifier(func(progress AnalysisProgress) {
				photo, err := store.GetPhoto(progress.PhotoID)
				if err != nil || photo.AnalysisStatus != progress.Status {
					t.Fatalf("event was emitted before persistence: %+v, %v", progress, err)
				}
				status, err := s.AnalysisStatus()
				if err != nil {
					t.Fatal(err)
				}
				if total := status.Pending + status.Processing + status.Retrying + status.Completed + status.Failed; total != int64(test.count) {
					t.Fatalf("inconsistent count: %+v", status)
				}
				maxRetrying = max(maxRetrying, status.Retrying)
			})
			s.processPhotoAnalysisBatch(ids)
			if maxRetrying != int64(test.count) {
				t.Fatalf("retrying count = %d, want %d", maxRetrying, test.count)
			}
			status, err := s.AnalysisStatus()
			if err != nil {
				t.Fatal(err)
			}
			if status.Retrying != 0 || status.Pending != 0 || status.Processing != 0 {
				t.Fatalf("active counts not cleared: %+v", status)
			}
			if test.failed && status.Failed != int64(test.count) || !test.failed && status.Completed != int64(test.count) {
				t.Fatalf("unexpected final status: %+v", status)
			}
		})
	}
}

func TestRestartResumesRetryingPhoto(t *testing.T) {
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "resume.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := repository.New(db.GORM())
	ids := createAnalysisTestPhotos(t, store, t.TempDir(), 1)
	if err := store.SetPhotoAnalysisStatus(ids[0], model.AnalysisRetrying, ""); err != nil {
		t.Fatal(err)
	}
	s := NewWithAnalyzer(store, testPhotoAnalyzer{})
	defer s.Close()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		status, err := s.AnalysisStatus()
		if err == nil && status.Completed == 1 && status.Retrying == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("retrying photo was not resumed")
}
