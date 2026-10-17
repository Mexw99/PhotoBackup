package service_test

import (
	"context"
	"errors"
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
	image := filepath.Join(source, "sample.png")
	contents := []byte("\x89PNG\r\n\x1a\nimage-data")
	if err := os.WriteFile(image, contents, 0600); err != nil {
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
	tag, err := s.AddTagToPhotos([]uint{photos[0].ID}, "  holiday  ")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"AND", "OR"} {
		found, err := s.SearchPhotos([]string{"HOLIDAY"}, mode)
		if err != nil || len(found) != 1 || len(found[0].Tags) != 1 {
			t.Fatalf("search %s: %+v, %v", mode, found, err)
		}
	}
	if err := s.RemoveTagFromPhoto(photos[0].ID, tag.ID); err != nil {
		t.Fatal(err)
	}
	found, err := s.SearchPhotos([]string{"holiday"}, "OR")
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
