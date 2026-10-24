package service_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"photo-backup/database"
	"photo-backup/repository"
	"photo-backup/service"
)

func newService(t *testing.T) *service.Service {
	t.Helper()
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return service.New(repository.New(db.GORM()))
}

// Exercise the new service boundary with real temporary files and SQLite.
func TestBackupTagAndSearch(t *testing.T) {
	s := newService(t)
	source, destination := t.TempDir(), t.TempDir()
	imagePath := filepath.Join(source, "sample.png")
	var encoded bytes.Buffer
	picture := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			picture.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	if err := png.Encode(&encoded, picture); err != nil {
		t.Fatal(err)
	}
	contents := encoded.Bytes()
	if err := os.WriteFile(imagePath, contents, 0600); err != nil {
		t.Fatal(err)
	}
	progress := 0
	summary, err := s.StartBackup(source, destination, func(current, total int, name string) {
		progress = current
		if total != 1 || name != "sample.png" {
			t.Errorf("unexpected progress: %d, %s", total, name)
		}
	})
	if err != nil || summary.Success != 1 || progress != 1 {
		t.Fatalf("backup: %+v, %v, progress %d", summary, err, progress)
	}
	data, err := os.ReadFile(filepath.Join(destination, "sample.png"))
	if err != nil || string(data) != string(contents) {
		t.Fatalf("destination data: %q, %v", data, err)
	}
	photos, err := s.ListPhotos()
	if err != nil || len(photos) != 1 {
		t.Fatalf("photos: %+v, %v", photos, err)
	}
	if photos[0].ColorTag != "" {
		t.Fatalf("color analysis should be disabled: %+v", photos[0])
	}
	if _, err := os.Stat(imagePath); !os.IsNotExist(err) {
		t.Fatalf("original remains after move: %v", err)
	}
	tag, err := s.AddTagToPhotos([]uint{photos[0].ID}, "  holiday  ")
	if err != nil {
		t.Fatal(err)
	}
	found, err := s.SearchPhotos([]string{"HOLIDAY"})
	if err != nil || len(found) != 1 || len(found[0].Tags) != 1 {
		t.Fatalf("search: %+v, %v", found, err)
	}
	if err := s.RemoveTagFromPhoto(photos[0].ID, tag.ID); err != nil {
		t.Fatal(err)
	}
	found, err = s.SearchPhotos([]string{"holiday"})
	if err != nil || len(found) != 0 {
		t.Fatalf("removed tag search: %+v, %v", found, err)
	}
}

func TestDatabaseOperationsUseApplicationContext(t *testing.T) {
	s := newService(t)
	ctx, cancel := context.WithCancel(context.Background())
	s.SetContext(ctx)
	cancel()
	_, err := s.ListPhotos()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context, got %v", err)
	}
}
