package database_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"photo-backup/database"
	"photo-backup/model"
)

func TestMigrationAddsAIColumnsDropsColorAndPreservesExistingData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.db")
	old, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE photos (id INTEGER PRIMARY KEY, original_name TEXT NOT NULL, original_path TEXT NOT NULL, backup_path TEXT NOT NULL, file_size INTEGER NOT NULL DEFAULT 0, color_tag TEXT NOT NULL DEFAULT '', backed_up_at DATETIME NOT NULL, created_at DATETIME, updated_at DATETIME)`,
		`INSERT INTO photos (id, original_name, original_path, backup_path, file_size, color_tag, backed_up_at) VALUES (1, 'old.jpg', 'source/old.jpg', 'backup/old.jpg', 12, 'สีแดง', '2026-01-01 10:00:00')`,
		`CREATE TABLE tags (id INTEGER PRIMARY KEY, name TEXT NOT NULL, created_at DATETIME, updated_at DATETIME)`,
		`INSERT INTO tags (id, name) VALUES (1, 'holiday')`,
		`CREATE TABLE photo_tags (photo_id INTEGER, tag_id INTEGER, PRIMARY KEY (photo_id, tag_id))`,
		`INSERT INTO photo_tags VALUES (1, 1)`,
		`CREATE TABLE files (idx INTEGER PRIMARY KEY, dest TEXT NOT NULL, filename TEXT NOT NULL, status TEXT NOT NULL)`,
		`INSERT INTO files VALUES (1, 'backup', 'old.jpg', 'backed_up')`,
		`CREATE TABLE settings (idx INTEGER PRIMARY KEY, source TEXT NOT NULL, dest TEXT NOT NULL)`,
		`INSERT INTO settings VALUES (1, 'source', 'backup')`,
	}
	for _, sql := range statements {
		if err := old.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	oldSQL, err := old.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := oldSQL.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := database.OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var photo model.Photo
	if err := db.GORM().Preload("Tags").First(&photo, 1).Error; err != nil {
		t.Fatal(err)
	}
	if photo.OriginalName != "old.jpg" || photo.FileSize != 12 || photo.AnalysisStatus != model.AnalysisPending {
		t.Fatalf("existing photo changed: %+v", photo)
	}
	if len(photo.Tags) != 1 || photo.Tags[0].Name != "holiday" {
		t.Fatalf("existing tags changed: %+v", photo.Tags)
	}
	if db.GORM().Migrator().HasColumn(&model.Photo{}, "color_tag") {
		t.Fatal("old color_tag column still exists")
	}
	for _, column := range []string{"description", "analysis_status", "analysis_error", "content_hash", "analyzed_at"} {
		if !db.GORM().Migrator().HasColumn(&model.Photo{}, column) {
			t.Fatalf("AI column %q was not created", column)
		}
	}
	var history model.FileRecord
	var setting model.Setting
	if err := db.GORM().First(&history).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.GORM().First(&setting).Error; err != nil {
		t.Fatal(err)
	}
	if history.Filename != "old.jpg" || setting.Source != "source" || setting.Dest != "backup" {
		t.Fatalf("midterm data changed: history=%+v, setting=%+v", history, setting)
	}
}

func TestNewDatabaseHasAIColumns(t *testing.T) {
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "new.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, column := range []string{"description", "analysis_status", "analysis_error", "content_hash", "analyzed_at"} {
		if !db.GORM().Migrator().HasColumn(&model.Photo{}, column) {
			t.Fatalf("new database is missing %q", column)
		}
	}
	if db.GORM().Migrator().HasColumn(&model.Photo{}, "color_tag") {
		t.Fatal("new database still has color_tag")
	}
}

func TestMigrationKeepsIndexesAndUniqueBackupPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "indexes.db")
	db, err := database.OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	photo := model.Photo{
		OriginalName: "one.jpg", OriginalPath: "source/one.jpg", BackupPath: "backup/one.jpg",
		BackedUpAt: now,
	}
	if err := db.GORM().Create(&photo).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = database.OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, name := range []string{"idx_photos_backup_path", "idx_photos_original_path"} {
		if !db.GORM().Migrator().HasIndex(&model.Photo{}, name) {
			t.Fatalf("missing index %q", name)
		}
	}
	duplicate := model.Photo{OriginalName: "two.jpg", OriginalPath: "source/two.jpg", BackupPath: "backup/one.jpg", BackedUpAt: now}
	if err := db.GORM().Create(&duplicate).Error; err == nil {
		t.Fatal("backup path unique constraint was lost")
	}
}
