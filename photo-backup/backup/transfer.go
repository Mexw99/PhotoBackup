package backup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var copyBufferPool = sync.Pool{
	New: func() any {
		buffer := make([]byte, 4*1024*1024)
		return &buffer
	},
}

type BackupResult struct {
	OriginalName string `json:"originalName"`
	OriginalPath string `json:"originalPath"`
	BackupPath   string `json:"backupPath"`
	FileSize     int64  `json:"fileSize"`
	Success      bool   `json:"success"`
	Skipped      bool   `json:"skipped"`
	Message      string `json:"message"`
	Error        string `json:"error"`
}

type BackupSummary struct {
	Total   int            `json:"total"`
	Success int            `json:"success"`
	Skipped int            `json:"skipped"`
	Failed  int            `json:"failed"`
	Results []BackupResult `json:"results"`
}

type ProgressCallback func(current int, total int, name string)

// CopyImage เตรียมไฟล์ปลายทางก่อน Service บันทึก DB และลบต้นทาง
func CopyImage(image ImageFile, destDir string) (BackupResult, error) {
	return copyImage(image, destDir, 0)
}

func CopyImageFromNumber(image ImageFile, destDir string, startNumber int) (BackupResult, error) {
	if startNumber < 0 {
		startNumber = 0
	}
	return copyImage(image, destDir, startNumber)
}

func copyImage(image ImageFile, destDir string, startNumber int) (BackupResult, error) {
	result := BackupResult{OriginalName: image.Name, OriginalPath: image.Path, FileSize: image.Size}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return result, err
	}
	source, err := os.Open(image.Path)
	if err != nil {
		return result, err
	}
	defer source.Close()

	for number := startNumber; ; number++ {
		name := image.Name
		if number > 0 {
			extension := filepath.Ext(image.Name)
			base := strings.TrimSuffix(image.Name, extension)
			name = fmt.Sprintf("%s (%d)%s", base, number, extension)
		}
		destination := filepath.Join(destDir, name)
		file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return result, err
		}

		buffer := copyBufferPool.Get().(*[]byte)
		written, copyErr := io.CopyBuffer(file, source, *buffer)
		copyBufferPool.Put(buffer)
		if copyErr == nil {
			copyErr = file.Sync()
		}
		closeErr := file.Close()
		if copyErr == nil {
			copyErr = closeErr
		}
		if copyErr == nil && written != image.Size {
			copyErr = fmt.Errorf("ไฟล์ต้นทางเปลี่ยนระหว่างสำรอง กรุณาลองใหม่")
		}
		if copyErr == nil {
			var equal bool
			equal, copyErr = EqualFiles(image.Path, destination)
			if copyErr == nil && !equal {
				copyErr = fmt.Errorf("ข้อมูลปลายทางไม่ตรงกับต้นทาง กรุณาลองใหม่")
			}
		}
		if copyErr != nil {
			_ = os.Remove(destination)
			return result, copyErr
		}
		result.BackupPath, result.Success = destination, true
		return result, nil
	}
}
