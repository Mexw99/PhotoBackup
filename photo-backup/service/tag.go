package service

import (
	"photo-backup/model"
	"photo-backup/repository"
)

func (s *Service) CountTags() (int64, error) {
	store, cancel := s.operation()
	defer cancel()
	return store.CountTags()
}

func (s *Service) ListTags() ([]model.Tag, error) {
	store, cancel := s.operation()
	defer cancel()
	return store.ListTags()
}

func (s *Service) AddTagToPhotos(ids []uint, name string) (*model.Tag, error) {
	store, cancel := s.operation()
	defer cancel()
	return store.AddTagToPhotos(ids, name)
}

func (s *Service) RemoveTagFromPhoto(photoID, tagID uint) error {
	store, cancel := s.operation()
	defer cancel()
	return store.RemoveTagFromPhoto(photoID, tagID)
}

func (s *Service) DeleteTag(tagID uint) error {
	store, cancel := s.operation()
	defer cancel()
	return store.DeleteTag(tagID)
}

func (s *Service) GetTagCloud() ([]repository.TagCount, error) {
	store, cancel := s.operation()
	defer cancel()
	return store.GetTagCloud()
}
