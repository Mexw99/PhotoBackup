package database_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"photo-backup/database"
	"photo-backup/model"
)

// จำลอง schema ที่ฟีเจอร์คำอธิบายเคยสร้างผ่าน GORM
type legacyPhoto struct {
	model.Photo
	Description string `gorm:"not null;default:''"`
}

func (legacyPhoto) TableName() string { return "photos" }

func TestMigrationPreservesExistingPhotosAndMidtermTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.db")
	old, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE photos (id INTEGER PRIMARY KEY, original_name TEXT NOT NULL, original_path TEXT NOT NULL, backup_path TEXT NOT NULL, file_size INTEGER NOT NULL DEFAULT 0, backed_up_at DATETIME NOT NULL, created_at DATETIME, updated_at DATETIME)`,
		`INSERT INTO photos (id, original_name, original_path, backup_path, backed_up_at) VALUES (1, 'old.jpg', 'source/old.jpg', 'backup/old.jpg', '2026-01-01 10:00:00')`,
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
	if photo.OriginalName != "old.jpg" || photo.ColorTag != "" {
		t.Fatalf("existing photo changed: %+v", photo)
	}
	if len(photo.Tags) != 1 || photo.Tags[0].Name != "holiday" {
		t.Fatalf("existing tags changed: %+v", photo.Tags)
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

func TestNewDatabaseHasDescriptionColumn(t *testing.T) {
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "new.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if !db.GORM().Migrator().HasColumn(&model.Photo{}, "description") {
		t.Fatal("new database is missing the AI description column")
	}
}

func TestSQLiteConfiguresBusyTimeoutAndWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pragma.db")
	db, err := database.OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var timeout int
	if err := db.GORM().Raw("PRAGMA busy_timeout").Scan(&timeout).Error; err != nil {
		t.Fatal(err)
	}
	if timeout < 5000 {
		t.Fatalf("busy_timeout = %d, want at least 5000ms", timeout)
	}
	var journalMode string
	if err := db.GORM().Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		t.Fatalf("journal_mode = %q, want WAL", journalMode)
	}
	sqlDB, err := db.GORM().DB()
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB.Stats().MaxOpenConnections != 1 {
		t.Fatalf("MaxOpenConnections = %d, want 1 so PRAGMA applies to every pooled connection", sqlDB.Stats().MaxOpenConnections)
	}
	// Close and reopen the database to force a genuinely new SQLite connection.
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = database.OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	sqlDB, err = db.GORM().DB()
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB.Stats().MaxOpenConnections != 1 {
		t.Fatalf("reopened MaxOpenConnections = %d, want 1", sqlDB.Stats().MaxOpenConnections)
	}
	var newTimeout int
	if err := db.GORM().Raw("PRAGMA busy_timeout").Scan(&newTimeout).Error; err != nil {
		t.Fatal(err)
	}
	var newJournalMode string
	if err := db.GORM().Raw("PRAGMA journal_mode").Scan(&newJournalMode).Error; err != nil {
		t.Fatal(err)
	}
	if newTimeout < 5000 || !strings.EqualFold(newJournalMode, "wal") {
		t.Fatalf("new pooled connection pragmas: timeout=%d journal=%q", newTimeout, newJournalMode)
	}
}

func TestMigrationClassifiesExistingPhotoTagsAsUser(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old-tags.db")
	old, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := old.AutoMigrate(&model.Photo{}, &model.Tag{}); err != nil {
		t.Fatal(err)
	}
	photo := model.Photo{OriginalName: "old.jpg", OriginalPath: "source/old.jpg", BackupPath: "backup/old.jpg", BackedUpAt: time.Now(), Tags: []model.Tag{{Name: "existing"}}}
	if err := old.Create(&photo).Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, err := old.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := database.OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var link model.PhotoTag
	if err := db.GORM().Where("photo_id = ? AND tag_id = ?", photo.ID, 1).First(&link).Error; err != nil {
		t.Fatal(err)
	}
	if link.Source != "user" {
		t.Fatalf("old tag source = %q, want user", link.Source)
	}
}

func TestMigrationAddsContentHashAndAIResultCacheWithoutChangingExistingData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ai-cache.db")
	old, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := old.AutoMigrate(&model.Photo{}, &model.Tag{}); err != nil {
		t.Fatal(err)
	}
	contentHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	photo := model.Photo{
		OriginalName: "old.jpg",
		OriginalPath: "source/old.jpg",
		BackupPath:   "backup/old.jpg",
		FileSize:     123,
		Description:  "คำอธิบายเดิม",
		BackedUpAt:   time.Now(),
		Tags:         []model.Tag{{Name: "holiday"}},
	}
	if err := old.Create(&photo).Error; err != nil {
		t.Fatal(err)
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

	if !db.GORM().Migrator().HasColumn(&model.Photo{}, "content_hash") {
		t.Fatal("migration did not add photos.content_hash")
	}
	if !db.GORM().Migrator().HasIndex(&model.Photo{}, "idx_photos_content_hash") {
		t.Fatal("migration did not add the content hash index")
	}
	if !db.GORM().Migrator().HasTable(&model.AIResult{}) {
		t.Fatal("migration did not add the ai_results table")
	}
	if !db.GORM().Migrator().HasIndex(&model.AIResult{}, "idx_ai_results_cache_key") {
		t.Fatal("migration did not add the AI cache unique index")
	}

	var preserved model.Photo
	if err := db.GORM().Preload("Tags").First(&preserved, photo.ID).Error; err != nil {
		t.Fatal(err)
	}
	if preserved.Description != "คำอธิบายเดิม" || len(preserved.Tags) != 1 || preserved.Tags[0].Name != "holiday" {
		t.Fatalf("migration changed existing photo data: %+v", preserved)
	}
	if preserved.ContentHash != nil {
		t.Fatalf("migration unexpectedly backfilled content hash: %v", *preserved.ContentHash)
	}

	result := model.AIResult{
		ContentHash:     contentHash,
		AnalysisProfile: "default-vision",
		Provider:        "test",
		Model:           "test-model",
		PromptVersion:   "v1",
		Description:     "คำอธิบายจาก Cache",
		TagsJSON:        `["holiday","ทะเล"]`,
	}
	if err := db.GORM().Create(&result).Error; err != nil {
		t.Fatal(err)
	}

	var loaded model.AIResult
	if err := db.GORM().First(&loaded, result.ID).Error; err != nil {
		t.Fatal(err)
	}
	if loaded.ContentHash != contentHash || loaded.AnalysisProfile != "default-vision" || loaded.TagsJSON != result.TagsJSON {
		t.Fatalf("AI result was not preserved: %+v", loaded)
	}

	duplicate := model.AIResult{
		ContentHash:     contentHash,
		AnalysisProfile: "default-vision",
		Provider:        "other-provider",
		Model:           "other-model",
		PromptVersion:   "v1",
		Description:     "duplicate",
		TagsJSON:        `[]`,
	}
	if err := db.GORM().Create(&duplicate).Error; err == nil {
		t.Fatal("expected duplicate AI cache key to be rejected")
	}
}

func databaseWithOldDescription(t *testing.T, viaGORM bool) (*database.Database, string, model.Photo) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "old-description.db")
	db, err := database.OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_ = viaGORM // OpenAt already creates the description column for the AI feature.
	now := time.Now()
	photo := model.Photo{
		OriginalName: "red.png", OriginalPath: "source/red.png", BackupPath: "backup/red.png",
		FileSize: 123, ColorTag: "สีแดง", SourceModifiedAt: now.Add(-time.Hour), BackedUpAt: now,
		Tags: []model.Tag{{Name: "สีแดง"}, {Name: "family"}},
	}
	if err := db.GORM().Create(&photo).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.GORM().Exec("UPDATE photos SET description = ? WHERE id = ?", "old text", photo.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.GORM().Create(&model.FileRecord{Dest: "backup", Filename: "red.png", Status: "backed_up"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.GORM().Create(&model.Setting{Source: "source", Dest: "backup"}).Error; err != nil {
		t.Fatal(err)
	}
	return db, path, photo
}

func checkPreservedData(t *testing.T, conn *gorm.DB, expected model.Photo) {
	t.Helper()
	var photo model.Photo
	if err := conn.Preload("Tags").First(&photo, expected.ID).Error; err != nil {
		t.Fatal(err)
	}
	if photo.OriginalName != expected.OriginalName || photo.OriginalPath != expected.OriginalPath || photo.BackupPath != expected.BackupPath || photo.FileSize != expected.FileSize || photo.ColorTag != expected.ColorTag || !photo.SourceModifiedAt.Equal(expected.SourceModifiedAt) || !photo.BackedUpAt.Equal(expected.BackedUpAt) {
		t.Fatalf("migration changed photo data: %+v", photo)
	}
	names := map[string]bool{}
	for _, tag := range photo.Tags {
		names[tag.Name] = true
	}
	if len(names) != 2 || !names["สีแดง"] || !names["family"] {
		t.Fatalf("migration changed photo/tag relationships: %+v", photo.Tags)
	}
	var history model.FileRecord
	var setting model.Setting
	if err := conn.First(&history).Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.First(&setting).Error; err != nil {
		t.Fatal(err)
	}
	if history.Dest != "backup" || history.Filename != "red.png" || history.Status != "backed_up" || setting.Source != "source" || setting.Dest != "backup" {
		t.Fatalf("migration changed history/settings: history=%+v, settings=%+v", history, setting)
	}
}

func TestMigrationKeepsDescriptionAndPreservesOtherData(t *testing.T) {
	for _, test := range []struct {
		name    string
		viaGORM bool
	}{
		{name: "unquoted column"},
		{name: "GORM quoted column", viaGORM: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			old, path, expected := databaseWithOldDescription(t, test.viaGORM)
			if err := old.Close(); err != nil {
				t.Fatal(err)
			}
			db, err := database.OpenAt(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			if !db.GORM().Migrator().HasColumn(&model.Photo{}, "description") {
				t.Fatal("description column was removed")
			}
			checkPreservedData(t, db.GORM(), expected)
			for _, name := range []string{"idx_photos_backup_path", "idx_photos_original_path"} {
				if !db.GORM().Migrator().HasIndex(&model.Photo{}, name) {
					t.Fatalf("migration removed photo index %q", name)
				}
			}
			duplicate := model.Photo{OriginalName: "other.png", OriginalPath: "other.png", BackupPath: expected.BackupPath, BackedUpAt: time.Now()}
			if err := db.GORM().Create(&duplicate).Error; err == nil {
				t.Fatal("backup path lost its unique constraint")
			}
			// เปิดครั้งต่อไปต้องยังเก็บ description ไว้
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = database.OpenAt(path)
			if err != nil {
				t.Fatal(err)
			}
			if !db.GORM().Migrator().HasColumn(&model.Photo{}, "description") {
				t.Fatal("description disappeared after reopening")
			}
			checkPreservedData(t, db.GORM(), expected)
		})
	}
}

func TestFailedMigrationRollsBackDescriptionRemoval(t *testing.T) {
	old, path, expected := databaseWithOldDescription(t, false)
	// ทำให้ AutoMigrate สร้าง unique index ไม่สำเร็จหลังขั้นตอนลบคอลัมน์
	if err := old.GORM().Migrator().DropIndex(&model.Tag{}, "idx_tags_name"); err != nil {
		t.Fatal(err)
	}
	if err := old.GORM().Exec("INSERT INTO tags (name) VALUES (?), (?)", "duplicate", "duplicate").Error; err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	if db, err := database.OpenAt(path); err == nil {
		_ = db.Close()
		t.Fatal("expected duplicate tags to prevent migration")
	}
	conn, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := conn.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if !conn.Migrator().HasColumn(&model.Photo{}, "description") {
		t.Fatal("failed migration committed the column removal")
	}
	var description string
	if err := conn.Raw("SELECT description FROM photos WHERE id = ?", expected.ID).Scan(&description).Error; err != nil {
		t.Fatal(err)
	}
	if description != "old text" {
		t.Fatalf("failed migration changed old data: %q", description)
	}
	checkPreservedData(t, conn, expected)
}
