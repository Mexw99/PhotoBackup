package service

import (
	"io/fs"
	"path/filepath"
	"strings"

	"photo-backup/backup"

	"golang.org/x/text/unicode/norm"
)

type backupNames struct {
	names map[string]string
}

func filenameKey(name string) string { return strings.ToLower(norm.NFC.String(name)) }

func (n *backupNames) duplicates(images []backup.ImageFile) map[string]string {
	duplicates := make(map[string]string)
	for _, image := range images {
		key := filenameKey(image.Name)
		if previous, exists := n.names[key]; exists {
			duplicates[image.Path] = previous
			continue
		}
		n.names[key] = image.Path
	}
	return duplicates
}

func (s *Service) backupNamesInDestination(dest string) (*backupNames, error) {
	names := &backupNames{names: make(map[string]string)}
	err := filepath.WalkDir(dest, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != dest && !entry.IsDir() {
			names.names[filenameKey(entry.Name())] = path
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	store, cancel := s.operation()
	defer cancel()
	paths, err := store.BackupPathsInDestination(dest)
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		names.names[filenameKey(filepath.Base(path))] = path
	}
	return names, nil
}
