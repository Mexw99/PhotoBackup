package service

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
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
