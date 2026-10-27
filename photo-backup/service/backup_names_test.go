package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackupFilenameOnlyPolicy(t *testing.T) {
	for _, test := range []struct {
		name             string
		names            []string
		success, skipped int
	}{
		{"identical content different filenames", []string{"one.png", "two.png"}, 2, 0},
		{"same filename different folders", []string{"a/photo.png", "b/photo.png"}, 1, 1},
		{"case insensitive across platforms", []string{"a/PHOTO.PNG", "b/photo.png"}, 1, 1},
		{"unicode normalized", []string{"a/caf\u00e9.png", "b/cafe\u0301.png"}, 1, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, _ := newMoveService(t)
			source, destination := t.TempDir(), t.TempDir()
			for _, name := range test.names {
				path := filepath.Join(source, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				writeMovePhoto(t, path)
			}
			result, err := s.StartBackup(source, destination, nil)
			if err != nil || result.Success != test.success || result.Skipped != test.skipped || result.Failed != 0 {
				t.Fatalf("%+v, %v", result, err)
			}
			for _, item := range result.Results {
				if item.Skipped {
					if _, err := os.Stat(item.OriginalPath); err != nil {
						t.Fatal("skipped source missing", err)
					}
					if item.Message == "" {
						t.Fatal("missing duplicate explanation")
					}
				}
			}
			if test.skipped > 0 {
				if result.Results[0].Skipped || !result.Results[1].Skipped {
					t.Fatalf("duplicate selection must be deterministic by sorted source path: %+v", result.Results)
				}
				if !strings.Contains(result.Results[1].Message, result.Results[0].OriginalPath) {
					t.Fatalf("duplicate message should identify the retained copy: %+v", result.Results[1])
				}
			}
		})
	}
}

func TestBackupSkipsSameFilenameDifferentContent(t *testing.T) {
	s, _ := newMoveService(t)
	source, dest := t.TempDir(), t.TempDir()
	path := filepath.Join(source, "photo.png")
	writeMovePhoto(t, path)
	if err := os.MkdirAll(filepath.Join(dest, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(dest, "nested", "PHOTO.PNG")
	if err := os.WriteFile(existing, []byte("different content"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := s.StartBackup(source, dest, nil)
	if err != nil || result.Skipped != 1 || result.Success != 0 || result.Failed != 0 {
		t.Fatalf("%+v, %v", result, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(existing); string(data) != "different content" {
		t.Fatal("existing content changed")
	}
}

func TestBackupThousandFilesAndDuplicateRerun(t *testing.T) {
	s, _ := newMoveService(t)
	source, dest := t.TempDir(), t.TempDir()
	data := writeMovePhoto(t, filepath.Join(source, "photo-0000.png"))
	for i := 1; i < 1000; i++ {
		if err := os.WriteFile(filepath.Join(source, fmt.Sprintf("photo-%04d.png", i)), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	updates := 0
	result, err := s.StartBackup(source, dest, func(current, total int, name string) {
		updates++
		if current != updates || total != 1000 {
			t.Errorf("invalid progress: %d/%d", current, total)
		}
	})
	if err != nil || result.Success != 1000 || result.Failed != 0 || updates != 1000 {
		t.Fatalf("counts=%d/%d/%d progress=%d error=%v", result.Success, result.Skipped, result.Failed, updates, err)
	}
	t.Logf("1000 small fixture files, move + SQLite: %s", time.Since(start))
	for i := 0; i < 1000; i++ {
		if err := os.WriteFile(filepath.Join(source, fmt.Sprintf("photo-%04d.png", i)), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	start = time.Now()
	result, err = s.StartBackup(source, dest, nil)
	if err != nil || result.Skipped != 1000 || result.Success != 0 || result.Failed != 0 {
		t.Fatalf("rerun counts=%d/%d/%d, %v", result.Success, result.Skipped, result.Failed, err)
	}
	photos, err := s.ListPhotos()
	if err != nil || len(photos) != 1000 {
		t.Fatalf("duplicate DB rows: %d, %v", len(photos), err)
	}
	left, _ := os.ReadDir(source)
	if len(left) != 1000 {
		t.Fatal("duplicate originals lost")
	}
	t.Logf("1000 name duplicates skipped: %s", time.Since(start))
}
