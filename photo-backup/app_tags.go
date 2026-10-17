package main

import "fmt"

func (a *App) GetTagCount() int64 {
	if a.service == nil {
		return 0
	}
	count, err := a.service.CountTags()
	if err != nil {
		return 0
	}
	return count
}

func (a *App) GetTags() []TagSummary {
	if a.service == nil {
		return []TagSummary{}
	}
	tags, err := a.service.ListTags()
	if err != nil {
		return []TagSummary{}
	}
	result := make([]TagSummary, 0, len(tags))
	for _, tag := range tags {
		result = append(result, TagSummary{ID: tag.ID, Name: tag.Name})
	}
	return result
}

func (a *App) AddTagToPhotos(photoIDs []uint, name string) (TagSummary, error) {
	if a.service == nil {
		return TagSummary{}, fmt.Errorf("database is not ready")
	}
	tag, err := a.service.AddTagToPhotos(photoIDs, name)
	if err != nil {
		return TagSummary{}, err
	}
	return TagSummary{ID: tag.ID, Name: tag.Name}, nil
}

func (a *App) RemoveTagFromPhoto(photoID, tagID uint) error {
	if a.service == nil {
		return fmt.Errorf("database is not ready")
	}
	return a.service.RemoveTagFromPhoto(photoID, tagID)
}

func (a *App) DeleteTag(tagID uint) error {
	if a.service == nil {
		return fmt.Errorf("database is not ready")
	}
	return a.service.DeleteTag(tagID)
}

func (a *App) GetTagCloud() ([]TagCloudItem, error) {
	if a.service == nil {
		return nil, fmt.Errorf("database is not ready")
	}
	tags, err := a.service.GetTagCloud()
	if err != nil {
		return nil, err
	}
	result := make([]TagCloudItem, 0, len(tags))
	for _, tag := range tags {
		result = append(result, TagCloudItem{ID: tag.ID, Name: tag.Name, PhotoCount: tag.PhotoCount})
	}
	return result, nil
}
