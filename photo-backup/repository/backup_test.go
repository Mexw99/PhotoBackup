package repository_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"photo-backup/database"
	"photo-backup/model"
	"photo-backup/repository"
)

func TestSaveBackupPersistsPhotoHistoryAndAIAnalysis(t *testing.T) {
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := repository.New(db.GORM())
	photo := model.Photo{
		OriginalName: "sample.jpg", OriginalPath: "source/sample.jpg",
		BackupPath: filepath.Join("backup", "sample_2.jpg"),
		FileSize:   20, BackedUpAt: time.Now(), SourceModifiedAt: time.Now(),
	}
	if err := repo.SaveBackup(&photo); err != nil {
		t.Fatal(err)
	}
	photos, err := repo.ListPhotos()
	if err != nil || len(photos) != 1 || len(photos[0].Tags) != 0 || photos[0].AnalysisStatus != model.AnalysisPending {
		t.Fatalf("saved photos: %+v, %v", photos, err)
	}
	analyzedAt := time.Now()
	if err := repo.SavePhotoAnalysis(photo.ID, "ภาพตัวอย่าง", []string{"ตัวอย่าง", "รูปภาพ"}, "hash", analyzedAt); err != nil {
		t.Fatal(err)
	}
	photos, err = repo.ListPhotos()
	if err != nil || len(photos) != 1 || photos[0].Description != "ภาพตัวอย่าง" || photos[0].AnalysisStatus != model.AnalysisCompleted || len(photos[0].Tags) != 2 {
		t.Fatalf("AI analysis was not saved: %+v, %v", photos, err)
	}
	var history []model.FileRecord
	if err := db.GORM().Find(&history).Error; err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Filename != "sample_2.jpg" || history[0].Dest != "backup" || history[0].Status != "backed_up" {
		t.Fatalf("unexpected history: %+v", history)
	}
	found, err := repo.FindBackup(photo.OriginalPath, "backup")
	if err != nil || found == nil || found.ID != photo.ID || !found.SourceModifiedAt.Equal(photo.SourceModifiedAt) {
		t.Fatalf("find saved backup: %+v, %v", found, err)
	}
	missing, err := repo.FindBackup("missing.jpg", "backup")
	if err != nil || missing != nil {
		t.Fatalf("missing backup: %+v, %v", missing, err)
	}
}

func TestSavePhotoAnalysisRejectsIncompleteAIResult(t *testing.T) {
	repo := newTestRepository(t)
	photo := createTestPhoto(t, repo, "incomplete.jpg")

	if err := repo.SavePhotoAnalysis(photo.ID, "", []string{"tag"}, "hash", time.Now()); err == nil {
		t.Fatal("expected empty description to be rejected")
	}
	if err := repo.SavePhotoAnalysis(photo.ID, "คำอธิบาย", []string{"  "}, "hash", time.Now()); err == nil {
		t.Fatal("expected empty tags to be rejected")
	}

	saved, err := repo.GetPhoto(photo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.AnalysisStatus != model.AnalysisPending || saved.Description != "" || len(saved.Tags) != 0 {
		t.Fatalf("incomplete result changed photo: %+v", saved)
	}
}

func TestFindBackupUsesHistoryForSelectedDestination(t *testing.T) {
	repo := newTestRepository(t)
	base := t.TempDir()
	source := filepath.Join(base, "source", "sample.jpg")
	now := time.Now()
	var first model.Photo
	for index, folder := range []string{"backup-A", "backup-B", "backup-A-other"} {
		photo := model.Photo{
			OriginalName: "sample.jpg", OriginalPath: source,
			BackupPath: filepath.Join(base, folder, "album", "sample.jpg"),
			BackedUpAt: now.Add(time.Duration(index) * time.Minute),
		}
		if err := repo.CreatePhoto(&photo); err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			first = photo
		}
	}
	found, err := repo.FindBackup(source, filepath.Join(base, "backup-A"))
	if err != nil || found == nil || found.ID != first.ID {
		t.Fatalf("history for backup-A: %+v, %v", found, err)
	}
	missing, err := repo.FindBackup(source, filepath.Join(base, "backup-C"))
	if err != nil || missing != nil {
		t.Fatalf("history for unused destination: %+v, %v", missing, err)
	}
}

func TestSaveBackupRollsBackWhenHistoryCannotBeSaved(t *testing.T) {
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := repository.New(db.GORM())
	if err := db.GORM().Migrator().DropTable(&model.FileRecord{}); err != nil {
		t.Fatal(err)
	}
	photo := model.Photo{
		OriginalName: "sample.jpg", OriginalPath: "source/sample.jpg",
		BackupPath: "backup/sample.jpg", BackedUpAt: time.Now(),
	}
	if err := repo.SaveBackup(&photo); err == nil {
		t.Fatal("expected history write to fail")
	}
	photoCount, _ := repo.CountPhotos()
	tagCount, _ := repo.CountTags()
	if photoCount != 0 || tagCount != 0 {
		t.Fatalf("transaction left partial data: photos=%d, tags=%d", photoCount, tagCount)
	}
}

func TestSaveBackupRepairsExistingPathAndKeepsUserEdits(t *testing.T) {
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "repair.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := repository.New(db.GORM())
	now := time.Now()
	original := model.Photo{
		OriginalName: "sample.jpg", OriginalPath: "source/sample.jpg",
		BackupPath: "backup/sample.jpg", BackedUpAt: now, SourceModifiedAt: now,
		FileSize: 10,
	}
	if err := repo.SaveBackup(&original); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddTagToPhotos([]uint{original.ID}, "family"); err != nil {
		t.Fatal(err)
	}
	repaired := model.Photo{
		OriginalName: original.OriginalName, OriginalPath: original.OriginalPath,
		BackupPath: original.BackupPath, FileSize: 25,
		BackedUpAt: now.Add(time.Hour), SourceModifiedAt: now.Add(time.Minute),
	}
	if err := repo.SaveBackup(&repaired); err != nil {
		t.Fatal(err)
	}
	photos, err := repo.ListPhotos()
	if err != nil || len(photos) != 1 {
		t.Fatalf("repaired photos: %+v, %v", photos, err)
	}
	photo := photos[0]
	if photo.ID != original.ID || repaired.ID != original.ID {
		t.Fatalf("repair replaced user data: %+v", photo)
	}
	if photo.FileSize != 25 || !photo.SourceModifiedAt.Equal(repaired.SourceModifiedAt) || !photo.BackedUpAt.Equal(repaired.BackedUpAt) || photo.AnalysisStatus != model.AnalysisPending || photo.Description != "" {
		t.Fatalf("repair metadata: %+v", photo)
	}
	if len(photo.Tags) != 1 || photo.Tags[0].Name != "family" {
		t.Fatalf("repair removed user tag: %+v", photo.Tags)
	}
	var historyCount int64
	if err := db.GORM().Model(&model.FileRecord{}).Count(&historyCount).Error; err != nil {
		t.Fatal(err)
	}
	if historyCount != 2 {
		t.Fatalf("history count = %d, want 2", historyCount)
	}

	otherSource := model.Photo{
		OriginalName: "other.jpg", OriginalPath: "other/other.jpg",
		BackupPath: original.BackupPath, BackedUpAt: time.Now(),
	}
	if err := repo.SaveBackup(&otherSource); !errors.Is(err, repository.ErrBackupPathConflict) {
		t.Fatalf("different source must be rejected: %v", err)
	}
}

func TestSettingsUpdateOneRowAndFilenameSearch(t *testing.T) {
	repo := newTestRepository(t)
	setting, err := repo.GetSettings()
	if err != nil || setting.Idx != 0 {
		t.Fatalf("empty settings: %+v, %v", setting, err)
	}
	if err := repo.SaveSettings("source-one", "backup-one"); err != nil {
		t.Fatal(err)
	}
	first, _ := repo.GetSettings()
	if err := repo.SaveSettings("source-two", "backup-two"); err != nil {
		t.Fatal(err)
	}
	second, err := repo.GetSettings()
	if err != nil || second.Idx != first.Idx || second.Source != "source-two" || second.Dest != "backup-two" {
		t.Fatalf("updated settings: %+v, %v", second, err)
	}
	photo1 := createTestPhoto(t, repo, "beach.jpg")
	photo2 := createTestPhoto(t, repo, "beach-holiday.jpg")
	if _, err := repo.AddTagToPhotos([]uint{photo2.ID}, "holiday"); err != nil {
		t.Fatal(err)
	}
	found, err := repo.SearchPhotos("BEACH", nil)
	if err != nil || len(found) != 2 {
		t.Fatalf("filename search: %+v, %v", found, err)
	}
	found, err = repo.SearchPhotos("BEACH", []string{"holiday"})
	if err != nil || len(found) != 1 || found[0].ID == photo1.ID {
		t.Fatalf("text and tag search: %+v, %v", found, err)
	}
	found, err = repo.SearchPhotos("garden", nil)
	if err != nil || len(found) != 0 {
		t.Fatalf("unmatched filename search: %+v, %v", found, err)
	}
}

func TestRemovingTagOnlyRemovesThatAssociation(t *testing.T) {
	repo := newTestRepository(t)
	photo1 := createTestPhoto(t, repo, "first.jpg")
	photo2 := createTestPhoto(t, repo, "second.jpg")
	tag, err := repo.AddTagToPhotos([]uint{photo1.ID, photo2.ID}, "family")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.RemoveTagFromPhoto(photo1.ID, tag.ID); err != nil {
		t.Fatal(err)
	}
	photos, err := repo.ListPhotos()
	if err != nil || len(photos) != 2 {
		t.Fatal(err)
	}
	for _, photo := range photos {
		if photo.ID == photo1.ID && len(photo.Tags) != 0 {
			t.Fatalf("tag still belongs to first photo: %+v", photo.Tags)
		}
		if photo.ID == photo2.ID && (len(photo.Tags) != 1 || photo.Tags[0].Name != "family") {
			t.Fatalf("second photo lost tag: %+v", photo.Tags)
		}
	}
	if err := repo.DeleteTag(tag.ID); err != nil {
		t.Fatal(err)
	}
	if count, _ := repo.CountTags(); count != 0 {
		t.Fatalf("deleted tag remains: %d", count)
	}
}
