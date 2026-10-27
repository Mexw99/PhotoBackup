package service_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeSamplePhoto(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	picture := image.NewRGBA(image.Rect(0, 0, 3, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			picture.Set(x, y, color.RGBA{G: 255, A: 255})
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
}

func sourceMustBeGone(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("source remains after move: %v", err)
	}
}

func restoreSource(t *testing.T, path string, data []byte, modified time.Time) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
}

func TestBackupMovesFilesAndSupportsChangedDestination(t *testing.T) {
	s := newService(t)
	source, destination := t.TempDir(), t.TempDir()
	path := filepath.Join(source, "trip", "green.png")
	writeSamplePhoto(t, path)
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.StartBackup(source, destination, nil)
	if err != nil || first.Success != 1 {
		t.Fatalf("first: %+v, %v", first, err)
	}
	if first.Results[0].BackupPath != filepath.Join(destination, "trip", "green.png") {
		t.Fatal(first.Results)
	}
	sourceMustBeGone(t, path)
	empty, err := s.StartBackup(source, destination, nil)
	if err != nil || empty.Total != 0 || len(empty.Results) != 0 {
		t.Fatalf("empty source: %+v, %v", empty, err)
	}

	// ชื่อซ้ำต้องคงต้นฉบับไว้ แม้เนื้อไฟล์จะเหมือนสำเนาเดิม
	restoreSource(t, path, contents, info.ModTime())
	second, err := s.StartBackup(source, destination, nil)
	if err != nil || second.Skipped != 1 || second.Success != 0 {
		t.Fatalf("repeat: %+v, %v", second, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	photos, err := s.ListPhotos()
	if err != nil || len(photos) != 1 {
		t.Fatalf("duplicate records: %+v, %v", photos, err)
	}
	newDestination := t.TempDir()
	restoreSource(t, path, contents, info.ModTime())
	third, err := s.StartBackup(source, newDestination, nil)
	if err != nil || third.Success != 1 || third.Skipped != 0 {
		t.Fatalf("new destination: %+v, %v", third, err)
	}
	if _, err := os.Stat(filepath.Join(newDestination, "trip", "green.png")); err != nil {
		t.Fatal(err)
	}
	sourceMustBeGone(t, path)
	restoreSource(t, path, contents, info.ModTime())
	backToFirst, err := s.StartBackup(source, destination, nil)
	if err != nil || backToFirst.Skipped != 1 || backToFirst.Success != 0 {
		t.Fatalf("return to first destination: %+v, %v", backToFirst, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestBackupKeepsSourceForRecordedNameEvenWhenCopyMissingOrTimeChanged(t *testing.T) {
	s := newService(t)
	source, destination := t.TempDir(), t.TempDir()
	path := filepath.Join(source, "green.png")
	writeSamplePhoto(t, path)
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.StartBackup(source, destination, nil)
	if err != nil || first.Success != 1 {
		t.Fatalf("first: %+v, %v", first, err)
	}
	if err := os.Remove(first.Results[0].BackupPath); err != nil {
		t.Fatal(err)
	}
	restoreSource(t, path, contents, info.ModTime())
	second, err := s.StartBackup(source, destination, nil)
	if err != nil || second.Skipped != 1 || second.Success != 0 {
		t.Fatalf("missing copy: %+v, %v", second, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	modified := time.Now().Add(time.Hour)
	restoreSource(t, path, contents, modified)
	third, err := s.StartBackup(source, destination, nil)
	if err != nil || third.Success != 0 || third.Skipped != 1 {
		t.Fatalf("changed source: %+v, %v", third, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestBackupMoveSkipsDuplicateDestination(t *testing.T) {
	s := newService(t)
	source, destination := t.TempDir(), t.TempDir()
	path := filepath.Join(source, "green.png")
	writeSamplePhoto(t, path)
	existing := filepath.Join(destination, "green.png")
	if err := os.WriteFile(existing, []byte("existing file"), 0600); err != nil {
		t.Fatal(err)
	}
	summary, err := s.StartBackup(source, destination, nil)
	if err != nil || summary.Success != 0 || summary.Skipped != 1 || summary.Results[0].BackupPath != "" {
		t.Fatalf("move with name collision: %+v, %v", summary, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("duplicate source should remain: %v", err)
	}
	oldData, err := os.ReadFile(existing)
	if err != nil || string(oldData) != "existing file" {
		t.Fatalf("existing file overwritten: %q, %v", oldData, err)
	}
	if _, err := os.Stat(filepath.Join(destination, "green (1).png")); !os.IsNotExist(err) {
		t.Fatalf("duplicate should not create a renamed copy: %v", err)
	}
}

func TestBackupInvalidImageReportsFailure(t *testing.T) {
	s := newService(t)
	source, destination := t.TempDir(), t.TempDir()
	path := filepath.Join(source, "broken.png")
	if err := os.WriteFile(path, []byte("not a PNG"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := s.StartBackup(source, destination, nil)
	if err != nil || result.Failed != 1 || result.Success != 0 {
		t.Fatalf("invalid: %+v, %v", result, err)
	}
	photos, err := s.ListPhotos()
	if err != nil || len(photos) != 0 {
		t.Fatalf("saved invalid photo: %+v, %v", photos, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("original removed")
	}
}
