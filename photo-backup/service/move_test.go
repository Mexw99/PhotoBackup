package service

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"photo-backup/database"
	"photo-backup/model"
	"photo-backup/repository"
)

func newMoveService(t *testing.T) (*Service, *database.Database) {
	t.Helper()
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "move.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewWithAnalyzer(repository.New(db.GORM()), nil), db
}

func writeMovePhoto(t *testing.T, path string) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 1, 1))
	picture.Set(0, 0, color.RGBA{R: 255, A: 255})
	var data bytes.Buffer
	if err := png.Encode(&data, picture); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestMoveDatabaseFailureKeepsSourceAndRemovesNewCopy(t *testing.T) {
	s, db := newMoveService(t)
	source, destination := t.TempDir(), t.TempDir()
	path := filepath.Join(source, "red.png")
	contents := writeMovePhoto(t, path)
	if err := db.GORM().Migrator().DropTable(&model.FileRecord{}); err != nil {
		t.Fatal(err)
	}
	s.removeSource = func(string) error {
		t.Fatal("source removal ran before SQLite was saved")
		return nil
	}
	summary, err := s.StartBackup(source, destination, nil)
	if err != nil || summary.Failed != 1 || summary.Success != 0 {
		t.Fatalf("database failure: %+v, %v", summary, err)
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, contents) {
		t.Fatalf("database failure changed source: %v", err)
	}
	files, err := os.ReadDir(destination)
	if err != nil || len(files) != 0 {
		t.Fatalf("new copy was not cleaned up: %+v, %v", files, err)
	}
	photos, err := s.ListPhotos()
	if err != nil || len(photos) != 0 {
		t.Fatalf("database failure left photo rows: %+v, %v", photos, err)
	}
}

func TestMoveSkipsDestinationRecordedInDatabase(t *testing.T) {
	s, db := newMoveService(t)
	source, destination := t.TempDir(), t.TempDir()
	path := filepath.Join(source, "red.png")
	writeMovePhoto(t, path)
	expectedDestination := filepath.Join(destination, "red.png")
	if err := db.GORM().Create(&model.Photo{
		OriginalName: "red.png", OriginalPath: "another-source/red.png",
		BackupPath: expectedDestination, FileSize: 10, BackedUpAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}

	summary, err := s.StartBackup(source, destination, nil)
	if err != nil || summary.Success != 0 || summary.Skipped != 1 {
		t.Fatalf("database duplicate: %+v, %v", summary, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("source should remain: %v", err)
	}
	if summary.Results[0].Message == "" {
		t.Fatal("database duplicate should explain why it was skipped")
	}
}

func TestMoveRetryFinishesSourceRemovalWithoutAnotherCopy(t *testing.T) {
	s, db := newMoveService(t)
	source, destination := t.TempDir(), t.TempDir()
	path := filepath.Join(source, "red.png")
	contents := writeMovePhoto(t, path)
	s.removeSource = func(string) error {
		var count int64
		if err := db.GORM().Model(&model.Photo{}).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("source removal ran before saving photo: count=%d, %v", count, err)
		}
		return os.ErrPermission
	}
	first, err := s.StartBackup(source, destination, nil)
	if err != nil || first.Failed != 1 || first.Success != 0 || first.Skipped != 0 || first.Results[0].BackupPath == "" {
		t.Fatalf("source removal failure: %+v, %v", first, err)
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, contents) {
		t.Fatalf("failed removal changed source: %v", err)
	}
	s.removeSource = os.Remove
	second, err := s.StartBackup(source, destination, nil)
	if err != nil || second.Skipped != 1 || second.Failed != 0 || second.Success != 0 || second.Results[0].BackupPath != first.Results[0].BackupPath {
		t.Fatalf("retry: %+v, %v", second, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("retry left source behind: %v", err)
	}
	files, err := os.ReadDir(destination)
	if err != nil || len(files) != 1 {
		t.Fatalf("retry created duplicate copies: %+v, %v", files, err)
	}
	var photoCount, historyCount int64
	db.GORM().Model(&model.Photo{}).Count(&photoCount)
	db.GORM().Model(&model.FileRecord{}).Count(&historyCount)
	if photoCount != 1 || historyCount != 1 {
		t.Fatalf("retry created duplicate records: photos=%d, history=%d", photoCount, historyCount)
	}
}

func TestMoveRetryVerifiesBytesBeforeDeletingSource(t *testing.T) {
	s, _ := newMoveService(t)
	source, destination := t.TempDir(), t.TempDir()
	path := filepath.Join(source, "red.png")
	contents := writeMovePhoto(t, path)
	s.removeSource = func(string) error { return os.ErrPermission }
	first, err := s.StartBackup(source, destination, nil)
	if err != nil || first.Failed != 1 {
		t.Fatalf("first attempt: %+v, %v", first, err)
	}
	changed := append([]byte(nil), contents...)
	changed[len(changed)-1] ^= 1
	if err := os.WriteFile(first.Results[0].BackupPath, changed, 0600); err != nil {
		t.Fatal(err)
	}
	s.removeSource = os.Remove
	second, err := s.StartBackup(source, destination, nil)
	if err != nil || second.Success != 1 || second.Skipped != 0 || second.Results[0].BackupPath == first.Results[0].BackupPath {
		t.Fatalf("retry accepted altered copy: %+v, %v", second, err)
	}
	actual, err := os.ReadFile(second.Results[0].BackupPath)
	if err != nil || !bytes.Equal(actual, contents) {
		t.Fatalf("retry did not preserve original bytes: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("successful retry left source: %v", err)
	}
}

func TestFinishMoveKeepsSourceWhenItChanged(t *testing.T) {
	s, _ := newMoveService(t)
	path := filepath.Join(t.TempDir(), "red.png")
	contents := writeMovePhoto(t, path)
	destination := filepath.Join(t.TempDir(), "red.png")
	if err := os.WriteFile(destination, contents, 0600); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("new source data"), 0600); err != nil {
		t.Fatal(err)
	}
	s.removeSource = func(string) error {
		t.Fatal("changed source must not be removed")
		return nil
	}
	if err := s.finishMove(path, destination, previous); err == nil {
		t.Fatal("expected changed source to stop removal")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "new source data" {
		t.Fatalf("changed source was lost: %q, %v", data, err)
	}
}

func TestFinishMoveKeepsSourceWhenDestinationDisappears(t *testing.T) {
	s, _ := newMoveService(t)
	path := filepath.Join(t.TempDir(), "red.png")
	contents := writeMovePhoto(t, path)
	previous, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	s.removeSource = func(string) error {
		t.Fatal("source must stay when destination is missing")
		return nil
	}
	if err := s.finishMove(path, filepath.Join(t.TempDir(), "missing.png"), previous); err == nil {
		t.Fatal("expected missing destination to stop source removal")
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, contents) {
		t.Fatalf("source was lost: %v", err)
	}
}

func TestMoveRetryDoesNotReuseSymlinkToSource(t *testing.T) {
	s, _ := newMoveService(t)
	source, destination := t.TempDir(), t.TempDir()
	path := filepath.Join(source, "red.png")
	contents := writeMovePhoto(t, path)
	s.removeSource = func(string) error { return os.ErrPermission }
	first, err := s.StartBackup(source, destination, nil)
	if err != nil || first.Failed != 1 {
		t.Fatalf("first attempt: %+v, %v", first, err)
	}
	if err := os.Remove(first.Results[0].BackupPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path, first.Results[0].BackupPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	s.removeSource = os.Remove
	second, err := s.StartBackup(source, destination, nil)
	if err != nil || second.Success != 1 || second.Skipped != 0 || second.Results[0].BackupPath == first.Results[0].BackupPath {
		t.Fatalf("reused symlink instead of preparing a file: %+v, %v", second, err)
	}
	actual, err := os.ReadFile(second.Results[0].BackupPath)
	if err != nil || !bytes.Equal(actual, contents) {
		t.Fatalf("moved bytes lost: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("source remains: %v", err)
	}
}
