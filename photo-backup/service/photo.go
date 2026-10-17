package service

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"

	"photo-backup/model"
)

func (s *Service) CountPhotos() (int64, error) {
	store, cancel := s.operation()
	defer cancel()
	return store.CountPhotos()
}

func (s *Service) RecentPhotos(limit int) ([]model.Photo, error) {
	store, cancel := s.operation()
	defer cancel()
	return store.RecentPhotos(limit)
}

func (s *Service) ListPhotos() ([]model.Photo, error) {
	store, cancel := s.operation()
	defer cancel()
	return store.ListPhotos()
}

func (s *Service) SearchPhotos(tags []string) ([]model.Photo, error) {
	store, cancel := s.operation()
	defer cancel()
	return store.SearchPhotosByTags(tags)
}

func (s *Service) SearchPhotoLibrary(query string, tags []string) ([]model.Photo, error) {
	store, cancel := s.operation()
	defer cancel()
	return store.SearchPhotos(query, tags)
}

// ReadImageDataURL returns an empty preview when the image is unavailable.
func ReadImageDataURL(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("data:%s;base64,%s", http.DetectContentType(data), base64.StdEncoding.EncodeToString(data))
}
