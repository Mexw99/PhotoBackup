package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverImagesFindsSupportedFilesRecursively(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"photo.JPG":         "jpg",
		"nested/photo.jpeg": "jpeg",
		"nested/photo.PNG":  "png",
		"notes.txt":         "not an image",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	images, err := DiscoverImages(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 3 {
		t.Fatalf("expected 3 images, got %d", len(images))
	}
}

func TestDiscoverImagesRejectsMissingDirectory(t *testing.T) {
	_, err := DiscoverImages(filepath.Join(t.TempDir(), "missing"))
	if err != ErrSourceDirectoryNotFound {
		t.Fatalf("expected ErrSourceDirectoryNotFound, got %v", err)
	}
}
