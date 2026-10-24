package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"photo-backup/model"
)

func TestCheckBackupReportsMissingAndSizeMismatch(t *testing.T) {
	s, db := newMoveService(t)
	destination := t.TempDir()

	okPath := filepath.Join(destination, "ok.png")
	okData := writeMovePhoto(t, okPath)
	missingPath := filepath.Join(destination, "missing.png")
	changedPath := filepath.Join(destination, "changed.png")
	if err := os.WriteFile(changedPath, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}

	photos := []model.Photo{
		{OriginalName: "ok.png", OriginalPath: "source/ok.png", BackupPath: okPath, FileSize: int64(len(okData)), BackedUpAt: time.Now()},
		{OriginalName: "missing.png", OriginalPath: "source/missing.png", BackupPath: missingPath, FileSize: 10, BackedUpAt: time.Now()},
		{OriginalName: "changed.png", OriginalPath: "source/changed.png", BackupPath: changedPath, FileSize: 100, BackedUpAt: time.Now()},
	}
	if err := db.GORM().Create(&photos).Error; err != nil {
		t.Fatal(err)
	}

	results, err := s.CheckBackup(destination)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("result count = %d, want 3", len(results))
	}
	statusByName := make(map[string]string, len(results))
	for _, result := range results {
		statusByName[result.Filename] = result.Status
	}
	if statusByName["ok.png"] != "OK" {
		t.Fatalf("ok result: %+v", results)
	}
	if statusByName["missing.png"] != "ALERT" || statusByName["changed.png"] != "ALERT" {
		t.Fatalf("alert results: %+v", results)
	}
}
