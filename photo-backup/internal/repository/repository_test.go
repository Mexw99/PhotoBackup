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

func TestSearchPhotosByTags(t *testing.T) {
	repo := newTestRepository(t)
	photo1 := createTestPhoto(t, repo, "beach1.jpg")
	photo2 := createTestPhoto(t, repo, "mountain1.jpg")
	photo3 := createTestPhoto(t, repo, "both.jpg")

	// photo1: "beach"
	if _, err := repo.AddTagToPhotos([]uint{photo1.ID}, "beach"); err != nil {
		t.Fatalf("add beach tag to photo1: %v", err)
	}
	// photo2: "mountain"
	if _, err := repo.AddTagToPhotos([]uint{photo2.ID}, "mountain"); err != nil {
		t.Fatalf("add mountain tag to photo2: %v", err)
	}
	// photo3: "beach" and "mountain"
	if _, err := repo.AddTagToPhotos([]uint{photo3.ID}, "beach"); err != nil {
		t.Fatalf("add beach tag to photo3: %v", err)
	}
	if _, err := repo.AddTagToPhotos([]uint{photo3.ID}, "mountain"); err != nil {
		t.Fatalf("add mountain tag to photo3: %v", err)
	}

	t.Run("EmptyQueryReturnsAllPhotos", func(t *testing.T) {
		res, err := repo.SearchPhotosByTags([]string{}, "OR")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 3 {
			t.Errorf("got %d photos, want 3", len(res))
		}
	})

	t.Run("SingleTagCaseInsensitiveWithSpaces", func(t *testing.T) {
		res, err := repo.SearchPhotosByTags([]string{"  BEACH  "}, "OR")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// photo1 and photo3 have "beach"
		if len(res) != 2 {
			t.Fatalf("got %d photos, want 2", len(res))
		}
		for _, p := range res {
			if p.ID != photo1.ID && p.ID != photo3.ID {
				t.Errorf("unexpected photo ID %d in results", p.ID)
			}
		}
	})

	t.Run("ModeORMatchesAnyTag", func(t *testing.T) {
		res, err := repo.SearchPhotosByTags([]string{"beach", "mountain"}, "OR")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// photo1, photo2, photo3 all match
		if len(res) != 3 {
			t.Errorf("got %d photos, want 3", len(res))
		}
	})

	t.Run("ModeANDMatchesAllTags", func(t *testing.T) {
		res, err := repo.SearchPhotosByTags([]string{"beach", "mountain"}, "AND")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Only photo3 has both "beach" and "mountain"
		if len(res) != 1 {
			t.Fatalf("got %d photos, want 1", len(res))
		}
		if res[0].ID != photo3.ID {
			t.Errorf("photo ID = %d, want %d", res[0].ID, photo3.ID)
		}
		if len(res[0].Tags) != 2 {
			t.Errorf("preloaded tags count = %d, want 2", len(res[0].Tags))
		}
	})

	t.Run("NotFoundReturnsEmptyWithoutError", func(t *testing.T) {
		res, err := repo.SearchPhotosByTags([]string{"nonexistent"}, "OR")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 0 {
			t.Errorf("got %d photos, want 0", len(res))
		}
	})
}
