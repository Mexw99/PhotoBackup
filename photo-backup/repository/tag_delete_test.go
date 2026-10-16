package repository_test

import (
	"errors"
	"path/filepath"
	"testing"

	"photo-backup/database"
	"photo-backup/model"
	"photo-backup/repository"
)

func TestDeleteTagPersistsAndPreservesPhotosAndOtherTags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tags.db")
	db, err := database.OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := repository.New(db.GORM())
	first := createTestPhoto(t, repo, "first.jpg")
	second := createTestPhoto(t, repo, "second.jpg")
	deletedTag, err := repo.AddTagToPhotos([]uint{first.ID, second.ID}, "holiday")
	if err != nil {
		t.Fatal(err)
	}
	keptTag, err := repo.AddTagToPhotos([]uint{first.ID}, "family")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteTag(deletedTag.ID); err != nil {
		t.Fatal(err)
	}

	// Reopen the SQLite file to verify this is persisted, not only hidden in UI.
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = database.OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	var tagRows, joinRows int64
	if err := db.GORM().Model(&model.Tag{}).Where("id = ?", deletedTag.ID).Count(&tagRows).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.GORM().Table("photo_tags").Where("tag_id = ?", deletedTag.ID).Count(&joinRows).Error; err != nil {
		t.Fatal(err)
	}
	if tagRows != 0 || joinRows != 0 {
		t.Fatalf("deleted tag still exists: tags=%d, photo_tags=%d", tagRows, joinRows)
	}
	repo = repository.New(db.GORM())
	photos, err := repo.ListPhotos()
	if err != nil || len(photos) != 2 {
		t.Fatalf("expected both photos to remain: %+v, %v", photos, err)
	}
	for _, photo := range photos {
		if photo.ID == first.ID {
			if len(photo.Tags) != 1 || photo.Tags[0].ID != keptTag.ID {
				t.Fatalf("unrelated tag was changed: %+v", photo.Tags)
			}
		} else if len(photo.Tags) != 0 {
			t.Fatalf("deleted tag remains on second photo: %+v", photo.Tags)
		}
	}
	tags, err := repo.ListTags()
	if err != nil || len(tags) != 1 || tags[0].ID != keptTag.ID {
		t.Fatalf("remaining tags: %+v, %v", tags, err)
	}
}

func TestDeleteTagHandlesUnusedAndMissingTags(t *testing.T) {
	repo := newTestRepository(t)
	tag, err := repo.CreateTag("unused")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteTag(0); !errors.Is(err, repository.ErrTagNotFound) {
		t.Fatalf("zero ID must not delete an existing tag: %v", err)
	}
	if err := repo.DeleteTag(tag.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint{0, tag.ID} {
		if err := repo.DeleteTag(id); !errors.Is(err, repository.ErrTagNotFound) {
			t.Fatalf("DeleteTag(%d): expected ErrTagNotFound, got %v", id, err)
		}
	}
}

func TestDeleteTagRollsBackAssociationsOnFailure(t *testing.T) {
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := repository.New(db.GORM())
	photo := createTestPhoto(t, repo, "photo.jpg")
	tag, err := repo.AddTagToPhotos([]uint{photo.ID}, "keep")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.GORM().Exec("CREATE TRIGGER prevent_tag_delete BEFORE DELETE ON tags BEGIN SELECT RAISE(ABORT, 'blocked'); END").Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteTag(tag.ID); err == nil {
		t.Fatal("expected the database to reject deletion")
	}
	photos, err := repo.ListPhotos()
	if err != nil || len(photos) != 1 || len(photos[0].Tags) != 1 || photos[0].Tags[0].ID != tag.ID {
		t.Fatalf("association should be restored by rollback: %+v, %v", photos, err)
	}
}
