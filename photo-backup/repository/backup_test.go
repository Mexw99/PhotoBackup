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

func TestSaveBackupPersistsPhotoHistoryAndColorTag(t *testing.T) {
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
		ColorTag: "สีแดง",
	}
	if err := repo.SaveBackup(&photo); err != nil {
		t.Fatal(err)
	}
	photos, err := repo.ListPhotos()
	if err != nil || len(photos) != 1 || len(photos[0].Tags) != 1 {
		t.Fatalf("saved photos: %+v, %v", photos, err)
	}
	if photos[0].ColorTag != "สีแดง" || photos[0].Tags[0].Name != "สีแดง" {
		t.Fatalf("color tag was not saved: %+v", photos[0])
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
	// กลับไปเลือก A ต้องพบประวัติ A แม้ว่าประวัติ B จะใหม่กว่า
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
	// ทำให้ขั้นตอนสุดท้ายล้มเหลว เพื่อตรวจว่า photo และ tag ไม่ค้างในฐานข้อมูล
	if err := db.GORM().Migrator().DropTable(&model.FileRecord{}); err != nil {
		t.Fatal(err)
	}
	photo := model.Photo{
		OriginalName: "sample.jpg", OriginalPath: "source/sample.jpg",
		BackupPath: "backup/sample.jpg", BackedUpAt: time.Now(), ColorTag: "สีฟ้า",
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
		FileSize: 10, ColorTag: "สีแดง",
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
		ColorTag: "สีเขียว",
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
	if photo.FileSize != 25 || !photo.SourceModifiedAt.Equal(repaired.SourceModifiedAt) || !photo.BackedUpAt.Equal(repaired.BackedUpAt) || photo.ColorTag != "สีเขียว" {
		t.Fatalf("repair metadata: %+v", photo)
	}
	names := make(map[string]bool)
	for _, tag := range photo.Tags {
		names[tag.Name] = true
	}
	if len(names) != 2 || !names["family"] || !names["สีเขียว"] || names["สีแดง"] {
		t.Fatalf("repair tags: %+v", photo.Tags)
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
		BackupPath: original.BackupPath, BackedUpAt: time.Now(), ColorTag: "สีฟ้า",
	}
	if err := repo.SaveBackup(&otherSource); !errors.Is(err, repository.ErrBackupPathConflict) {
		t.Fatalf("different source must be rejected: %v", err)
	}
	photos, err = repo.ListPhotos()
	if err != nil || len(photos) != 1 || photos[0].OriginalPath != original.OriginalPath || photos[0].ColorTag != "สีเขียว" {
		t.Fatalf("conflict changed existing row: %+v, %v", photos, err)
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
	if _, err := repo.AddTagToPhotos([]uint{photo2.ID}, "สีเขียว"); err != nil {
		t.Fatal(err)
	}
	found, err := repo.SearchPhotos("BEACH", nil)
	if err != nil || len(found) != 2 {
		t.Fatalf("filename search: %+v, %v", found, err)
	}
	// ชื่อไฟล์ต้องตรงกับคำค้นและมีแท็กที่เลือกด้วย
	found, err = repo.SearchPhotos("BEACH", []string{"สีเขียว"})
	if err != nil || len(found) != 1 || found[0].ID == photo1.ID {
		t.Fatalf("text and tag search: %+v, %v", found, err)
	}
	found, err = repo.SearchPhotos("garden", nil)
	if err != nil || len(found) != 0 {
		t.Fatalf("unmatched filename search: %+v, %v", found, err)
	}
}

func TestRemovingColorTagsClearsSavedColor(t *testing.T) {
	repo := newTestRepository(t)
	for _, name := range []string{"first.jpg", "second.jpg"} {
		photo := model.Photo{
			OriginalName: name, OriginalPath: "source/" + name,
			BackupPath: "backup/" + name, BackedUpAt: time.Now(), ColorTag: "สีแดง",
		}
		if err := repo.SaveBackup(&photo); err != nil {
			t.Fatal(err)
		}
	}
	photos, err := repo.ListPhotos()
	if err != nil || len(photos) != 2 {
		t.Fatalf("saved photos: %+v, %v", photos, err)
	}
	tagID := photos[0].Tags[0].ID
	if err := repo.RemoveTagFromPhoto(photos[0].ID, tagID); err != nil {
		t.Fatal(err)
	}
	photos, err = repo.ListPhotos()
	if err != nil || photos[0].ColorTag != "" || photos[1].ColorTag != "สีแดง" {
		t.Fatalf("removed color from wrong photo: %+v, %v", photos, err)
	}
	if err := repo.DeleteTag(tagID); err != nil {
		t.Fatal(err)
	}
	photos, err = repo.ListPhotos()
	if err != nil {
		t.Fatal(err)
	}
	for _, photo := range photos {
		if photo.ColorTag != "" || len(photo.Tags) != 0 {
			t.Fatalf("deleted color remains: %+v", photo)
		}
	}
}
