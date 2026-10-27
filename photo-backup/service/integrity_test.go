package service_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"photo-backup/database"
	"photo-backup/model"
	"photo-backup/repository"
	"photo-backup/service"
)

func TestCheckBackupReportsMissingSizeAndContentMismatches(t *testing.T) {
	destination := t.TempDir()
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "integrity.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := service.New(repository.New(db.GORM()))

	okPath := filepath.Join(destination, "ok.png")
	if err := os.WriteFile(okPath, []byte("correct bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	changedPath := filepath.Join(destination, "changed.png")
	if err := os.WriteFile(changedPath, []byte("wrong--bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	sizePath := filepath.Join(destination, "size.png")
	if err := os.WriteFile(sizePath, []byte("short"), 0600); err != nil {
		t.Fatal(err)
	}

	correctHash := sha256.Sum256([]byte("correct bytes"))
	wrongExpectedHash := sha256.Sum256([]byte("other bytes!"))
	correctHashText := hex.EncodeToString(correctHash[:])
	wrongHashText := hex.EncodeToString(wrongExpectedHash[:])
	photos := []model.Photo{
		{OriginalName: "ok.png", OriginalPath: "source/ok.png", BackupPath: okPath, FileSize: 13, ContentHash: &correctHashText, BackedUpAt: time.Now()},
		{OriginalName: "changed.png", OriginalPath: "source/changed.png", BackupPath: changedPath, FileSize: 12, ContentHash: &wrongHashText, BackedUpAt: time.Now()},
		{OriginalName: "size.png", OriginalPath: "source/size.png", BackupPath: sizePath, FileSize: 100, BackedUpAt: time.Now()},
		{OriginalName: "missing.png", OriginalPath: "source/missing.png", BackupPath: filepath.Join(destination, "missing.png"), FileSize: 20, BackedUpAt: time.Now()},
	}
	if err := db.GORM().Create(&photos).Error; err != nil {
		t.Fatal(err)
	}

	results, err := s.CheckBackup(destination)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 {
		t.Fatalf("got %d results, want 4: %+v", len(results), results)
	}
	statusByName := make(map[string]string, len(results))
	messageByName := make(map[string]string, len(results))
	for _, result := range results {
		statusByName[result.Filename] = result.Status
		messageByName[result.Filename] = result.Message
	}
	if statusByName["ok.png"] != "OK" {
		t.Fatalf("expected intact file to be OK: %+v", results)
	}
	for _, name := range []string{"changed.png", "size.png", "missing.png"} {
		if statusByName[name] != "ALERT" {
			t.Fatalf("expected %s to be an alert: %+v", name, results)
		}
		if messageByName[name] == "" {
			t.Fatalf("expected per-file explanation for %s", name)
		}
	}
}
