package service

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"photo-backup/analysis"
	"photo-backup/database"
	"photo-backup/model"
	"photo-backup/repository"
)

type testPhotoAnalyzer struct{}

func (testPhotoAnalyzer) AnalyzeImage(context.Context, string) (analysis.Result, error) {
	return analysis.Result{Description: "ภาพทดสอบจาก AI", Tags: []string{"ทดสอบ", "รูปภาพ"}}, nil
}

type testBatchPhotoAnalyzer struct {
	batchCalls int
}

type retryBatchPhotoAnalyzer struct {
	mu            sync.Mutex
	batchCalls    int
	failedBatches int
}

type incompleteBatchPhotoAnalyzer struct {
	batchCalls  int
	singleCalls int
}

func (a *testBatchPhotoAnalyzer) AnalyzeImage(context.Context, string) (analysis.Result, error) {
	return analysis.Result{}, fmt.Errorf("ไม่ควรเรียกทีละรูป")
}

func (a *testBatchPhotoAnalyzer) AnalyzeImages(_ context.Context, images []analysis.ImageInput) (map[uint]analysis.Result, error) {
	a.batchCalls++
	results := make(map[uint]analysis.Result, len(images))
	for _, image := range images {
		results[image.ID] = analysis.Result{
			Description: fmt.Sprintf("ผลของรูป %d", image.ID),
			Tags:        []string{fmt.Sprintf("tag-%d", image.ID)},
		}
	}
	return results, nil
}

func (a *retryBatchPhotoAnalyzer) AnalyzeImage(_ context.Context, path string) (analysis.Result, error) {
	return analysis.Result{Description: "ผลสำรองรายรูป " + filepath.Base(path), Tags: []string{"สำเร็จ"}}, nil
}

func (a *retryBatchPhotoAnalyzer) AnalyzeImages(_ context.Context, images []analysis.ImageInput) (map[uint]analysis.Result, error) {
	a.mu.Lock()
	a.batchCalls++
	call := a.batchCalls
	a.mu.Unlock()
	if call <= a.failedBatches {
		return nil, fmt.Errorf("จำลอง API ล้มเหลวครั้งที่ %d", call)
	}
	results := make(map[uint]analysis.Result, len(images))
	for _, image := range images {
		results[image.ID] = analysis.Result{Description: fmt.Sprintf("ผลชุดของรูป %d", image.ID), Tags: []string{"สำเร็จ"}}
	}
	return results, nil
}

func (a *retryBatchPhotoAnalyzer) Calls() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.batchCalls
}

func (a *incompleteBatchPhotoAnalyzer) AnalyzeImage(_ context.Context, path string) (analysis.Result, error) {
	a.singleCalls++
	return analysis.Result{Description: "ผลวิเคราะห์รายรูป " + filepath.Base(path), Tags: []string{"สำเร็จ"}}, nil
}

func (a *incompleteBatchPhotoAnalyzer) AnalyzeImages(_ context.Context, images []analysis.ImageInput) (map[uint]analysis.Result, error) {
	a.batchCalls++
	return map[uint]analysis.Result{
		images[0].ID: {Description: "ผลไม่ครบชุด", Tags: []string{"ไม่ครบ"}},
	}, nil
}

func TestBackupQueuesAIAnalysisAfterCopy(t *testing.T) {
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "analysis.db"))
	if err != nil {
		t.Fatal(err)
	}
	s := NewWithAnalyzer(repository.New(db.GORM()), testPhotoAnalyzer{})
	t.Cleanup(func() {
		s.Close()
		_ = db.Close()
	})

	source, destination := t.TempDir(), t.TempDir()
	path := filepath.Join(source, "sample.png")
	picture := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			picture.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, picture); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	result, err := s.StartBackup(source, destination, nil)
	if err != nil || result.Success != 1 {
		t.Fatalf("backup: %+v, %v", result, err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		photos, listErr := s.ListPhotos()
		if listErr == nil && len(photos) == 1 && photos[0].AnalysisStatus == "completed" {
			if photos[0].Description != "ภาพทดสอบจาก AI" || len(photos[0].Tags) != 2 {
				t.Fatalf("unexpected AI result: %+v", photos[0])
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	photos, _ := s.ListPhotos()
	t.Fatalf("AI queue did not finish: %+v", photos)
}

func TestBatchAnalysisSavesEachPhotoByID(t *testing.T) {
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "batch-analysis.db"))
	if err != nil {
		t.Fatal(err)
	}
	analyzer := &testBatchPhotoAnalyzer{}
	s := NewWithAnalyzer(repository.New(db.GORM()), analyzer)
	t.Cleanup(func() {
		s.Close()
		_ = db.Close()
	})

	destination := t.TempDir()
	store := repository.New(db.GORM())
	ids := make([]uint, 0, 5)
	for index := 1; index <= 5; index++ {
		path := filepath.Join(destination, fmt.Sprintf("photo-%d.png", index))
		picture := image.NewRGBA(image.Rect(0, 0, 2, 2))
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(file, picture); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		photo := &model.Photo{
			OriginalName:   filepath.Base(path),
			OriginalPath:   path,
			BackupPath:     path,
			FileSize:       1,
			AnalysisStatus: model.AnalysisPending,
			BackedUpAt:     time.Now(),
		}
		if err := store.CreatePhoto(photo); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, photo.ID)
	}

	s.processPhotoAnalysisBatch(ids)
	if analyzer.batchCalls != 1 {
		t.Fatalf("expected one batch call, got %d", analyzer.batchCalls)
	}
	photos, err := s.ListPhotos()
	if err != nil {
		t.Fatal(err)
	}
	if len(photos) != len(ids) {
		t.Fatalf("expected %d photos, got %d", len(ids), len(photos))
	}
	for _, photo := range photos {
		expected := fmt.Sprintf("ผลของรูป %d", photo.ID)
		if photo.AnalysisStatus != model.AnalysisCompleted || photo.Description != expected || len(photo.Tags) != 1 {
			t.Fatalf("unexpected photo analysis: %+v", photo)
		}
	}
}

func TestBatchAnalysisRetriesBeforeSavingCompleteResults(t *testing.T) {
	t.Setenv("AI_MAX_RETRIES", "2")
	t.Setenv("AI_RETRY_BASE_DELAY_MS", "0")
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "retry-batch.db"))
	if err != nil {
		t.Fatal(err)
	}
	analyzer := &retryBatchPhotoAnalyzer{failedBatches: 2}
	s := NewWithAnalyzer(repository.New(db.GORM()), analyzer)
	t.Cleanup(func() {
		s.Close()
		_ = db.Close()
	})

	ids := createAnalysisTestPhotos(t, repository.New(db.GORM()), t.TempDir(), 5)
	s.processPhotoAnalysisBatch(ids)
	if analyzer.Calls() != 3 {
		t.Fatalf("batch calls = %d, want 3 after two failures", analyzer.Calls())
	}
	photos, err := s.ListPhotos()
	if err != nil {
		t.Fatal(err)
	}
	for _, photo := range photos {
		if photo.AnalysisStatus != model.AnalysisCompleted || len(photo.Tags) == 0 {
			t.Fatalf("photo was not completed with a tag: %+v", photo)
		}
	}
}

func TestBatchAnalysisFallsBackToSinglePhotosWhenBatchIsIncomplete(t *testing.T) {
	t.Setenv("AI_MAX_RETRIES", "1")
	t.Setenv("AI_RETRY_BASE_DELAY_MS", "0")
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "incomplete-batch.db"))
	if err != nil {
		t.Fatal(err)
	}
	analyzer := &incompleteBatchPhotoAnalyzer{}
	s := NewWithAnalyzer(repository.New(db.GORM()), analyzer)
	t.Cleanup(func() {
		s.Close()
		_ = db.Close()
	})

	ids := createAnalysisTestPhotos(t, repository.New(db.GORM()), t.TempDir(), 5)
	s.processPhotoAnalysisBatch(ids)
	if analyzer.batchCalls != 2 {
		t.Fatalf("batch calls = %d, want 2 before fallback", analyzer.batchCalls)
	}
	if analyzer.singleCalls != len(ids) {
		t.Fatalf("single calls = %d, want %d", analyzer.singleCalls, len(ids))
	}
	photos, err := s.ListPhotos()
	if err != nil {
		t.Fatal(err)
	}
	for _, photo := range photos {
		if photo.AnalysisStatus != model.AnalysisCompleted || photo.Description == "" || len(photo.Tags) == 0 {
			t.Fatalf("fallback did not complete photo with a tag: %+v", photo)
		}
	}
}

func TestAnalysisQueueProcessesTenPhotosWithoutMissingResults(t *testing.T) {
	t.Setenv("AI_ANALYSIS_WORKERS", "4")
	t.Setenv("AI_RETRY_BASE_DELAY_MS", "0")
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "ten-photos.db"))
	if err != nil {
		t.Fatal(err)
	}
	analyzer := &retryBatchPhotoAnalyzer{}
	s := NewWithAnalyzer(repository.New(db.GORM()), analyzer)
	t.Cleanup(func() {
		s.Close()
		_ = db.Close()
	})

	ids := createAnalysisTestPhotos(t, repository.New(db.GORM()), t.TempDir(), 10)
	for _, id := range ids {
		s.enqueuePhotoAnalysis(id)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status, statusErr := s.AnalysisStatus()
		if statusErr == nil && status.Completed == int64(len(ids)) && status.Pending == 0 && status.Processing == 0 && status.Failed == 0 {
			photos, listErr := s.ListPhotos()
			if listErr != nil {
				t.Fatal(listErr)
			}
			for _, photo := range photos {
				if len(photo.Tags) == 0 || photo.Description == "" {
					t.Fatalf("photo %d is missing analysis data: %+v", photo.ID, photo)
				}
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	status, _ := s.AnalysisStatus()
	t.Fatalf("ten-photo queue did not complete cleanly: %+v", status)
}

func TestAnalysisQueueProcessesOneThousandPhotosWithoutMissingResults(t *testing.T) {
	t.Setenv("AI_ANALYSIS_WORKERS", "4")
	t.Setenv("AI_RETRY_BASE_DELAY_MS", "0")
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "one-thousand-photos.db"))
	if err != nil {
		t.Fatal(err)
	}
	analyzer := &retryBatchPhotoAnalyzer{}
	s := NewWithAnalyzer(repository.New(db.GORM()), analyzer)
	t.Cleanup(func() {
		s.Close()
		_ = db.Close()
	})

	ids := createAnalysisTestPhotos(t, repository.New(db.GORM()), t.TempDir(), 1000)
	for _, id := range ids {
		s.enqueuePhotoAnalysis(id)
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		status, statusErr := s.AnalysisStatus()
		if statusErr == nil && status.Completed == int64(len(ids)) && status.Pending == 0 && status.Processing == 0 && status.Failed == 0 {
			photos, listErr := s.ListPhotos()
			if listErr != nil || len(photos) != len(ids) {
				t.Fatalf("one-thousand-photo results = %d, want %d, error = %v", len(photos), len(ids), listErr)
			}
			for _, photo := range photos {
				if photo.Description == "" || len(photo.Tags) == 0 {
					t.Fatalf("photo %d is missing analysis data: %+v", photo.ID, photo)
				}
			}
			if analyzer.Calls() < len(ids)/analysisBatchSize {
				t.Fatalf("batch calls = %d, want at least %d", analyzer.Calls(), len(ids)/analysisBatchSize)
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	status, _ := s.AnalysisStatus()
	t.Fatalf("one-thousand-photo queue did not complete cleanly: %+v", status)
}

func createAnalysisTestPhotos(t *testing.T, store repository.Store, directory string, count int) []uint {
	t.Helper()
	ids := make([]uint, 0, count)
	for index := 1; index <= count; index++ {
		path := filepath.Join(directory, fmt.Sprintf("photo-%d.png", index))
		if err := os.WriteFile(path, tinyTestPNG(), 0600); err != nil {
			t.Fatal(err)
		}
		photo := &model.Photo{
			OriginalName: filepath.Base(path), OriginalPath: path, BackupPath: path,
			FileSize: 1, AnalysisStatus: model.AnalysisPending, BackedUpAt: time.Now(),
		}
		if err := store.CreatePhoto(photo); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, photo.ID)
	}
	return ids
}

func tinyTestPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0xf0,
		0x1f, 0x00, 0x05, 0x00, 0x01, 0xff, 0x89, 0x99,
		0x3d, 0x1d, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
		0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
}
