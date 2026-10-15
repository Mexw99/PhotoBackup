package backup

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var ErrSourceDirectoryNotFound = errors.New("source directory does not exist")

// ImageFile is the file information needed by the Backup page before copying.
// Paths are returned by the operating system, so this works on Windows and macOS.
type ImageFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

var supportedExtensions = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
}

// DiscoverImages finds supported image files recursively under root.
func DiscoverImages(root string) ([]ImageFile, error) {
	if strings.TrimSpace(root) == "" {
		return nil, ErrSourceDirectoryNotFound
	}

	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrSourceDirectoryNotFound
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, ErrSourceDirectoryNotFound
	}

	files := make([]ImageFile, 0)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		extension := strings.ToLower(filepath.Ext(entry.Name()))
		if _, ok := supportedExtensions[extension]; !ok {
			return nil
		}

		fileInfo, err := entry.Info()
		if err != nil {
			return err
		}
		files = append(files, ImageFile{
			Name: entry.Name(),
			Path: path,
			Size: fileInfo.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Path) < strings.ToLower(files[j].Path)
	})
	return files, nil
}
