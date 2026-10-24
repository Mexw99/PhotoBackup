package service

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"strconv"
	"sync"

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

var thumbnailCache sync.Map

func ReadThumbnailDataURL(path string) string {
	if path == "" {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	cacheKey := path + "|" + strconv.FormatInt(info.ModTime().UnixNano(), 10) + "|" + strconv.FormatInt(info.Size(), 10)
	if cached, ok := thumbnailCache.Load(cacheKey); ok {
		return cached.(string)
	}
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	source, _, err := image.Decode(file)
	if err != nil {
		return ""
	}
	const maxWidth, maxHeight = 320, 240
	sourceBounds := source.Bounds()
	width, height := sourceBounds.Dx(), sourceBounds.Dy()
	if width <= 0 || height <= 0 {
		return ""
	}
	scale := 1.0
	if width > maxWidth || height > maxHeight {
		widthScale := float64(maxWidth) / float64(width)
		heightScale := float64(maxHeight) / float64(height)
		scale = widthScale
		if heightScale < scale {
			scale = heightScale
		}
	}
	destWidth, destHeight := max(1, int(float64(width)*scale)), max(1, int(float64(height)*scale))
	destination := image.NewRGBA(image.Rect(0, 0, destWidth, destHeight))
	for y := 0; y < destHeight; y++ {
		for x := 0; x < destWidth; x++ {
			sourceX := sourceBounds.Min.X + x*width/destWidth
			sourceY := sourceBounds.Min.Y + y*height/destHeight
			destination.Set(x, y, source.At(sourceX, sourceY))
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, destination, &jpeg.Options{Quality: 75}); err != nil {
		return ""
	}
	thumbnail := fmt.Sprintf("data:image/jpeg;base64,%s", base64.StdEncoding.EncodeToString(encoded.Bytes()))
	thumbnailCache.Store(cacheKey, thumbnail)
	return thumbnail
}
