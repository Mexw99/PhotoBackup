package service

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"sync"

	"photo-backup/internal/imageguard"
	"photo-backup/model"
	"photo-backup/repository"
)

const thumbnailMaxDimension = 320

type thumbnailCacheEntry struct {
	signature string
	data      string
}

var thumbnailCache = struct {
	sync.RWMutex
	items map[string]thumbnailCacheEntry
}{
	items: make(map[string]thumbnailCacheEntry),
}

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

func (s *Service) ListPhotosByIDs(ids []uint) ([]model.Photo, error) {
	if len(ids) > 100 {
		return nil, fmt.Errorf("โหลดข้อมูลที่เปลี่ยนแปลงได้ครั้งละไม่เกิน 100 รูป")
	}
	store, cancel := s.operation()
	defer cancel()
	return store.ListPhotosByIDs(ids)
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

func (s *Service) SearchPhotoPage(query string, tags []string, page, size int) (repository.PhotoPage, error) {
	if page < 1 || page > 1000000 || size < 1 || size > 60 {
		return repository.PhotoPage{}, fmt.Errorf("หน้าต้องเริ่มที่ 1 และแสดงได้ไม่เกิน 60 รูปต่อหน้า")
	}
	store, cancel := s.operation()
	defer cancel()
	return store.SearchPhotoPage(query, tags, page, size)
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

func ReadImageThumbnailDataURL(path string) string {
	if path == "" {
		return ""
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		return ""
	}
	signature := fmt.Sprintf("%d:%d", fileInfo.Size(), fileInfo.ModTime().UnixNano())
	thumbnailCache.RLock()
	cached, found := thumbnailCache.items[path]
	thumbnailCache.RUnlock()
	if found && cached.signature == signature {
		return cached.data
	}
	imageguard.Decode.Lock()
	defer imageguard.Decode.Unlock()
	thumbnailCache.RLock()
	cached, found = thumbnailCache.items[path]
	thumbnailCache.RUnlock()
	if found && cached.signature == signature {
		return cached.data
	}

	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	config, _, err := image.DecodeConfig(file)
	_ = file.Close()
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return ""
	}
	if int64(config.Width)*int64(config.Height) > 40000000 {
		return ""
	}

	file, err = os.Open(path)
	if err != nil {
		return ""
	}
	picture, _, decodeErr := image.Decode(file)
	_ = file.Close()
	if decodeErr != nil {
		return ""
	}

	scaled := resizeThumbnail(picture, thumbnailMaxDimension)
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, scaled, &jpeg.Options{Quality: 75}); err != nil {
		return ""
	}
	data := fmt.Sprintf("data:image/jpeg;base64,%s", base64.StdEncoding.EncodeToString(encoded.Bytes()))
	thumbnailCache.Lock()
	if len(thumbnailCache.items) >= 2048 {
		for cachedPath := range thumbnailCache.items {
			delete(thumbnailCache.items, cachedPath)
			break
		}
	}
	thumbnailCache.items[path] = thumbnailCacheEntry{signature: signature, data: data}
	thumbnailCache.Unlock()
	return data
}

func resizeThumbnail(source image.Image, maxDimension int) *image.RGBA {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= maxDimension && height <= maxDimension {
		maxDimension = max(width, height)
	}
	scale := float64(maxDimension) / float64(max(width, height))
	newWidth := max(1, int(float64(width)*scale))
	newHeight := max(1, int(float64(height)*scale))
	destination := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			sourceX := bounds.Min.X + x*width/newWidth
			sourceY := bounds.Min.Y + y*height/newHeight
			destination.Set(x, y, source.At(sourceX, sourceY))
		}
	}
	return destination
}
