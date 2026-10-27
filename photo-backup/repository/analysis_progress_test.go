package repository_test

import (
	"testing"

	"photo-backup/model"
)

func TestAnalysisCountsIncludeRetryingAndResumeOnlyActivePhotos(t *testing.T) {
	repo := newTestRepository(t)
	for _, status := range []string{model.AnalysisPending, model.AnalysisProcessing, model.AnalysisRetrying, model.AnalysisCompleted, model.AnalysisFailed} {
		photo := createTestPhoto(t, repo, status+".jpg")
		if err := repo.SetPhotoAnalysisStatus(photo.ID, status, ""); err != nil {
			t.Fatal(err)
		}
	}
	counts, err := repo.AnalysisCounts()
	if err != nil {
		t.Fatal(err)
	}
	if counts.Pending != 1 || counts.Processing != 1 || counts.Retrying != 1 || counts.Completed != 1 || counts.Failed != 1 {
		t.Fatalf("unexpected counts: %+v", counts)
	}
	ids, err := repo.PendingAnalysisPhotoIDs()
	if err != nil || len(ids) != 3 {
		t.Fatalf("resume IDs: %v, %v", ids, err)
	}
}

func TestListPhotosByIDsDoesNotReturnEntireLibrary(t *testing.T) {
	repo := newTestRepository(t)
	one := createTestPhoto(t, repo, "one.jpg")
	createTestPhoto(t, repo, "two.jpg")
	if _, err := repo.AddTagToPhotos([]uint{one.ID}, "cat"); err != nil {
		t.Fatal(err)
	}
	photos, err := repo.ListPhotosByIDs([]uint{one.ID, one.ID, 999})
	if err != nil || len(photos) != 1 || photos[0].ID != one.ID || len(photos[0].Tags) != 1 {
		t.Fatalf("delta photos: %+v, %v", photos, err)
	}
	photos, err = repo.ListPhotosByIDs(nil)
	if err != nil || len(photos) != 0 {
		t.Fatalf("empty IDs returned photos: %+v, %v", photos, err)
	}
}
