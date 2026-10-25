package repository_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"photo-backup/database"
	"photo-backup/model"
	"photo-backup/repository"
)

func TestAIResultCacheUpsertAndAppliesToSameContent(t *testing.T) {
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "ai.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := repository.New(db.GORM())
	hash := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	photo1 := createTestPhoto(t, repo, "one.jpg")
	photo2 := createTestPhoto(t, repo, "two.jpg")
	if err := db.GORM().Model(&model.Photo{}).Where("id IN ?", []uint{photo1.ID, photo2.ID}).Update("content_hash", hash).Error; err != nil {
		t.Fatal(err)
	}
	first := &model.AIResult{ContentHash: hash, AnalysisProfile: "vision-tags", Provider: "test", Model: "fake", PromptVersion: "v1", Description: "ทะเล", TagsJSON: `[` + `"ทะเล"` + `]`}
	if err := repo.SaveAIResultAndApply(first, []string{"ทะเล", "วันหยุด"}); err != nil {
		t.Fatal(err)
	}
	second := &model.AIResult{ContentHash: hash, AnalysisProfile: "vision-tags", Provider: "test", Model: "fake-v2", PromptVersion: "v1", Description: "ทะเลและภูเขา", TagsJSON: `[]`}
	if err := repo.SaveAIResultAndApply(second, []string{"ทะเล", "ภูเขา"}); err != nil {
		t.Fatal(err)
	}
	cached, err := repo.FindAIResult(hash, "vision-tags", "v1")
	if err != nil || cached == nil || cached.Description != "ทะเลและภูเขา" {
		t.Fatalf("cache upsert failed: %+v, %v", cached, err)
	}
	photos, err := repo.ListPhotos()
	if err != nil {
		t.Fatal(err)
	}
	for _, photo := range photos {
		if photo.Description != "ทะเลและภูเขา" || len(photo.Tags) < 2 {
			t.Fatalf("cache result was not applied to photo %d: %+v", photo.ID, photo)
		}
	}
	if !json.Valid([]byte(cached.TagsJSON)) {
		t.Fatal("cache TagsJSON is invalid")
	}
}

func TestAIJobUniqueKeyAndResume(t *testing.T) {
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := repository.New(db.GORM())
	photo := createTestPhoto(t, repo, "job.jpg")
	job := &model.AIJob{PhotoID: photo.ID, ContentHash: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", AnalysisProfile: "vision-tags", PromptVersion: "v1", Status: "queued"}
	created, err := repo.CreateAIJobIfMissing(job)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := repo.CreateAIJobIfMissing(&model.AIJob{PhotoID: photo.ID, ContentHash: job.ContentHash, AnalysisProfile: job.AnalysisProfile, PromptVersion: job.PromptVersion, Status: "queued"})
	if err != nil || duplicate.ID != created.ID {
		t.Fatalf("duplicate job was created: first=%+v second=%+v err=%v", created, duplicate, err)
	}
	claimed, err := repo.ClaimNextAIJob(created.CreatedAt)
	if err != nil || claimed == nil || claimed.Status != "processing" || claimed.AttemptCount != 1 {
		t.Fatalf("claim failed: %+v, %v", claimed, err)
	}
	if err := repo.ResetProcessingAIJobs(); err != nil {
		t.Fatal(err)
	}
	counts, err := repo.AIJobCounts()
	if err != nil || counts["queued"] != 1 {
		t.Fatalf("resume failed: %#v, %v", counts, err)
	}
}

func TestFailedAIJobIsRequeuedWithCurrentPhotoMetadata(t *testing.T) {
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "retry.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := repository.New(db.GORM())
	photo := createTestPhoto(t, repo, "retry.jpg")
	job := &model.AIJob{PhotoID: photo.ID, ContentHash: "old", AnalysisProfile: "vision-tags", PromptVersion: "v1", Provider: "local-portable", Model: "old", Status: "failed", AttemptCount: 4, LastError: "worker was offline"}
	if err := db.GORM().Create(job).Error; err != nil {
		t.Fatal(err)
	}
	updated, err := repo.CreateAIJobIfMissing(&model.AIJob{PhotoID: photo.ID, ContentHash: "new", AnalysisProfile: "vision-tags", PromptVersion: "v1", Provider: "local-hybrid", Model: "new", Status: "queued"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "queued" || updated.ContentHash != "new" || updated.AttemptCount != 0 || updated.LastError != "" {
		t.Fatalf("failed job was not safely requeued: %+v", updated)
	}
}

func TestConsumeAIRequestPersistsDailyLimit(t *testing.T) {
	db, err := database.OpenAt(filepath.Join(t.TempDir(), "usage.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := repository.New(db.GORM())
	allowed, err := repo.ConsumeAIRequest("test", "fake", 2)
	if err != nil || !allowed {
		t.Fatalf("first request: allowed=%v err=%v", allowed, err)
	}
	allowed, err = repo.ConsumeAIRequest("test", "fake", 2)
	if err != nil || !allowed {
		t.Fatalf("second request: allowed=%v err=%v", allowed, err)
	}
	allowed, err = repo.ConsumeAIRequest("test", "fake", 2)
	if err != nil || allowed {
		t.Fatalf("third request exceeded limit: allowed=%v err=%v", allowed, err)
	}
	var count int64
	if err := db.GORM().Table("ai_usages").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("daily usage rows=%d err=%v", count, err)
	}
}
