package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"photo-backup/database"
	"photo-backup/repository"
	"photo-backup/service"
)

// ทดสอบเส้นทาง API เดียวกับที่หน้า Vue เรียก โดยใช้ SQLite/รูปในโฟลเดอร์ชั่วคราว
func TestProgressDemoFlow(t *testing.T) {
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "demo.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app := NewApp()
	app.database, app.service = db, service.NewWithAnalyzer(repository.New(db.GORM()), nil)
	source, destination := t.TempDir(), t.TempDir()
	images := make(map[string][]byte)
	for _, name := range []string{"red.png", "green.png", "blue.png"} {
		data, err := os.ReadFile(filepath.Join("demo", "source", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, name), data, 0600); err != nil {
			t.Fatal(err)
		}
		images[name] = data
	}
	if err := app.SaveBackupSettings(source, destination); err != nil {
		t.Fatal(err)
	}
	settings, err := app.GetBackupSettings()
	if err != nil || settings.SourcePath != source || settings.DestPath != destination {
		t.Fatalf("settings: %+v, %v", settings, err)
	}
	first, err := app.StartBackup(source, destination)
	if err != nil || first.Success != 3 || first.Failed != 0 {
		t.Fatalf("backup: %+v, %v", first, err)
	}
	if app.GetPhotoCount() != 3 || app.GetTagCount() != 0 {
		t.Fatal("expected 3 photos and no automatic color tags")
	}
	photos, err := app.GetTaggedPhotos()
	if err != nil || len(photos) != 3 || photos[0].AnalysisStatus != "pending" {
		t.Fatalf("expected pending AI analysis: %+v, %v", photos, err)
	}
	for _, result := range first.Results {
		if _, err := os.Stat(result.OriginalPath); !os.IsNotExist(err) {
			t.Fatalf("source still exists after move: %s (%v)", result.OriginalPath, err)
		}
		data, err := os.ReadFile(result.BackupPath)
		if err != nil || !bytes.Equal(data, images[result.OriginalName]) {
			t.Fatalf("destination differs from original: %s (%v)", result.BackupPath, err)
		}
	}
	photos, err = app.SearchPhotoLibrary("red", nil)
	if err != nil || len(photos) != 1 || photos[0].Thumbnail == "" {
		t.Fatalf("search/preview: %+v, %v", photos, err)
	}
	redTag, err := app.AddTagToPhotos([]uint{photos[0].ID}, "แมว")
	if err != nil {
		t.Fatal(err)
	}
	for _, search := range []struct {
		name  string
		query string
		tags  []string
		count int
	}{
		{"filename without tags", "RED", nil, 1},
		{"one selected tag", "", []string{"แมว"}, 1},
		{"filename and matching tag", "red", []string{"แมว"}, 1},
		{"filename and different tag", "red", []string{"ไม่มีแท็กนี้"}, 0},
		{"no tags show all photos", "", nil, 3},
	} {
		t.Run(search.name, func(t *testing.T) {
			found, err := app.SearchPhotoLibrary(search.query, search.tags)
			if err != nil || len(found) != search.count {
				t.Fatalf("search returned %d photos, want %d: %v", len(found), search.count, err)
			}
		})
	}
	if err := app.DeleteTag(redTag.ID); err != nil {
		t.Fatal(err)
	}
	second, err := app.StartBackup(source, destination)
	if err != nil || second.Total != 0 || second.Success != 0 || second.Failed != 0 {
		t.Fatalf("repeat: %+v, %v", second, err)
	}
	if app.GetTagCount() != 0 {
		t.Fatal("deleted tag was not removed")
	}
}
