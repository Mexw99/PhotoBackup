package main

import (
	"fmt"

	"photo-backup/model"
	"photo-backup/service"
)

func (a *App) GetPhotoCount() int64 {
	if a.service == nil {
		return 0
	}
	count, err := a.service.CountPhotos()
	if err != nil {
		return 0
	}
	return count
}

func (a *App) GetImageBase64(filePath string) string {
	return service.ReadImageDataURL(filePath)
}

func (a *App) GetRecentPhotos(limit int) []PhotoSummary {
	if a.service == nil {
		return []PhotoSummary{}
	}
	photos, err := a.service.RecentPhotos(limit)
	if err != nil {
		return []PhotoSummary{}
	}
	result := make([]PhotoSummary, 0, len(photos))
	for _, photo := range photos {
		analyzedAt := ""
		if photo.AnalyzedAt != nil {
			analyzedAt = photo.AnalyzedAt.Format("2006-01-02 15:04:05")
		}
		result = append(result, PhotoSummary{
			ID:             photo.ID,
			OriginalName:   photo.OriginalName,
			BackupPath:     photo.BackupPath,
			BackedUpAt:     photo.BackedUpAt.Format("2006-01-02 15:04:05"),
			Thumbnail:      service.ReadImageDataURL(photo.BackupPath),
			Description:    photo.Description,
			AnalysisStatus: photo.AnalysisStatus,
			AnalysisError:  photo.AnalysisError,
			AnalyzedAt:     analyzedAt,
		})
	}
	return result
}

func (a *App) GetTaggedPhotos() ([]TaggedPhotoSummary, error) {
	if a.service == nil {
		return nil, fmt.Errorf("database is not ready")
	}
	photos, err := a.service.ListPhotos()
	if err != nil {
		return nil, err
	}
	return taggedPhotoSummaries(photos), nil
}

func (a *App) SearchPhotos(tagNames []string) ([]TaggedPhotoSummary, error) {
	if a.service == nil {
		return nil, fmt.Errorf("database is not ready")
	}
	photos, err := a.service.SearchPhotos(tagNames)
	if err != nil {
		return nil, err
	}
	return taggedPhotoSummaries(photos), nil
}

func (a *App) SearchPhotoLibrary(query string, tagNames []string) ([]TaggedPhotoSummary, error) {
	if a.service == nil {
		return nil, fmt.Errorf("database is not ready")
	}
	photos, err := a.service.SearchPhotoLibrary(query, tagNames)
	if err != nil {
		return nil, err
	}
	return taggedPhotoSummaries(photos), nil
}

// Keep JSON formatting in the presentation layer, shared by gallery and search.
func taggedPhotoSummaries(photos []model.Photo) []TaggedPhotoSummary {
	result := make([]TaggedPhotoSummary, 0, len(photos))
	for _, photo := range photos {
		analyzedAt := ""
		if photo.AnalyzedAt != nil {
			analyzedAt = photo.AnalyzedAt.Format("2006-01-02 15:04:05")
		}
		tags := make([]TagSummary, 0, len(photo.Tags))
		for _, tag := range photo.Tags {
			tags = append(tags, TagSummary{ID: tag.ID, Name: tag.Name})
		}
		result = append(result, TaggedPhotoSummary{
			ID:             photo.ID,
			OriginalName:   photo.OriginalName,
			BackupPath:     photo.BackupPath,
			BackedUpAt:     photo.BackedUpAt.Format("2006-01-02 15:04:05"),
			Thumbnail:      service.ReadImageDataURL(photo.BackupPath),
			Description:    photo.Description,
			AnalysisStatus: photo.AnalysisStatus,
			AnalysisError:  photo.AnalysisError,
			AnalyzedAt:     analyzedAt,
			Tags:           tags,
		})
	}
	return result
}
