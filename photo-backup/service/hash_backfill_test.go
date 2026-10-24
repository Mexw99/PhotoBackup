package service_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"photo-backup/database"
	"photo-backup/model"
	"photo-backup/repository"
	"photo-backup/service"
)

func TestBackfillContentHashesUsesBackupPathAndSkipsMissingFiles(t *testing.T) {
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "backfill.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := repository.New(db.GORM())
	backupPath := filepath.Join(t.TempDir(), "photo.png")
	if err := os.WriteFile(backupPath, []byte("backup bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	photos := []model.Photo{
		{OriginalName: "present.png", OriginalPath: "source/present.png", BackupPath: backupPath, FileSize: 12, BackedUpAt: time.Now()},
		{OriginalName: "missing.png", OriginalPath: "source/missing.png", BackupPath: filepath.Join(t.TempDir(), "missing.png"), FileSize: 1, BackedUpAt: time.Now()},
	}
	for index := range photos {
		if err := repo.CreatePhoto(&photos[index]); err != nil {
			t.Fatal(err)
		}
	}
	s := service.New(repo)
	count, err := s.BackfillContentHashes(10)
	if err != nil || count != 1 {
		t.Fatalf("backfill count=%d err=%v", count, err)
	}
	loaded, err := repo.ListPhotos()
	if err != nil {
		t.Fatal(err)
	}
	for _, photo := range loaded {
		if photo.OriginalName == "present.png" && (photo.ContentHash == nil || *photo.ContentHash == "") {
			t.Fatal("existing BackupPath did not receive a hash")
		}
		if photo.OriginalName == "missing.png" && photo.ContentHash != nil {
			t.Fatal("missing BackupPath should remain NULL")
		}
	}
}
