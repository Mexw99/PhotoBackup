package repository_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"photo-backup/internal/database"
	"photo-backup/internal/models"
	"photo-backup/internal/repository"
)

func newTestRepository(t *testing.T) *repository.Repository {
	t.Helper()

	db, err := database.OpenAt(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})

	return repository.New(db.GORM())
}

func createTestPhoto(t *testing.T, repo *repository.Repository, name string) models.Photo {
	t.Helper()

	photo := models.Photo{
		OriginalName: name,
		OriginalPath: filepath.Join("source", name),
		BackupPath:   filepath.Join(t.TempDir(), name),
		FileSize:     100,
		BackedUpAt:   time.Now(),
	}

	if err := repo.CreatePhoto(&photo); err != nil {
		t.Fatalf("create photo %q: %v", name, err)
	}

	return photo
}

func TestAddTagToPhotosTrimsNameAndAssociatesMultiplePhotos(t *testing.T) {
	repo := newTestRepository(t)
	photo1 := createTestPhoto(t, repo, "one.jpg")
	photo2 := createTestPhoto(t, repo, "two.jpg")

	tag, err := repo.AddTagToPhotos(
		[]uint{photo1.ID, photo2.ID},
		"  beach  ",
	)
	if err != nil {
		t.Fatalf("AddTagToPhotos returned error: %v", err)
	}
	if tag.Name != "beach" {
		t.Fatalf("tag name = %q, want %q", tag.Name, "beach")
	}

	photos, err := repo.ListPhotos()
	if err != nil {
		t.Fatalf("ListPhotos returned error: %v", err)
	}

	for _, photo := range photos {
		if len(photo.Tags) != 1 {
			t.Errorf("photo %q has %d tags, want 1", photo.OriginalName, len(photo.Tags))
			continue
		}
		if photo.Tags[0].Name != "beach" {
			t.Errorf("photo %q tag = %q, want %q",
				photo.OriginalName, photo.Tags[0].Name, "beach")
		}
	}
}

func TestAddTagToPhotosRejectsEmptyNameAndNoPhotos(t *testing.T) {
	repo := newTestRepository(t)
	photo := createTestPhoto(t, repo, "one.jpg")

	_, err := repo.AddTagToPhotos([]uint{photo.ID}, "   ")
	if !errors.Is(err, repository.ErrEmptyTagName) {
		t.Errorf("empty-name error = %v, want ErrEmptyTagName", err)
	}

	_, err = repo.AddTagToPhotos(nil, "beach")
	if !errors.Is(err, repository.ErrNoPhotoIDs) {
		t.Errorf("no-photo error = %v, want ErrNoPhotoIDs", err)
	}
}

func TestRemoveTagFromPhotoRemovesOnlyThatPhotoAssociation(t *testing.T) {
	repo := newTestRepository(t)
	photo1 := createTestPhoto(t, repo, "one.jpg")
	photo2 := createTestPhoto(t, repo, "two.jpg")

	tag, err := repo.AddTagToPhotos([]uint{photo1.ID, photo2.ID}, "beach")
	if err != nil {
		t.Fatalf("AddTagToPhotos returned error: %v", err)
	}

	if err := repo.RemoveTagFromPhoto(photo1.ID, tag.ID); err != nil {
		t.Fatalf("RemoveTagFromPhoto returned error: %v", err)
	}

	photos, err := repo.ListPhotos()
	if err != nil {
		t.Fatalf("ListPhotos returned error: %v", err)
	}

	for _, photo := range photos {
		switch photo.ID {
		case photo1.ID:
			if len(photo.Tags) != 0 {
				t.Errorf("photo one still has %d tags, want 0", len(photo.Tags))
			}
		case photo2.ID:
			if len(photo.Tags) != 1 || photo.Tags[0].Name != "beach" {
				t.Errorf("photo two should still have the beach tag; tags = %#v", photo.Tags)
			}
		}
	}
}

func TestGetTagCloudCountsAndSortsByPhotoCount(t *testing.T) {
	repo := newTestRepository(t)
	photo1 := createTestPhoto(t, repo, "one.jpg")
	photo2 := createTestPhoto(t, repo, "two.jpg")
	photo3 := createTestPhoto(t, repo, "three.jpg")

	if _, err := repo.AddTagToPhotos(
		[]uint{photo1.ID, photo2.ID, photo3.ID},
		"beach",
	); err != nil {
		t.Fatalf("add beach tag: %v", err)
	}

	if _, err := repo.AddTagToPhotos(
		[]uint{photo1.ID},
		"family",
	); err != nil {
		t.Fatalf("add family tag: %v", err)
	}

	tags, err := repo.GetTagCloud()
	if err != nil {
		t.Fatalf("GetTagCloud returned error: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("GetTagCloud returned %d tags, want 2", len(tags))
	}

	if tags[0].Name != "beach" || tags[0].PhotoCount != 3 {
		t.Errorf("first tag = %#v, want beach with 3 photos", tags[0])
	}
	if tags[1].Name != "family" || tags[1].PhotoCount != 1 {
		t.Errorf("second tag = %#v, want family with 1 photo", tags[1])
	}
}
