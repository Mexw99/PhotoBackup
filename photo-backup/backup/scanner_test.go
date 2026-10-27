package backup

import (
	"errors"
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

func TestCopyImagePreparesDestinationBeforeSourceRemoval(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	testFile := filepath.Join(srcDir, "pic.jpg")
	if err := os.WriteFile(testFile, []byte("image-data"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := CopyImage(ImageFile{Name: "pic.jpg", Path: testFile, Size: 10}, destDir)
	if err != nil || !result.Success {
		t.Fatalf("prepare destination: %+v, %v", result, err)
	}

	// 1. ตรวจสอบว่าไฟล์ปลายทางมีอยู่จริง
	expectedDest := filepath.Join(destDir, "pic.jpg")
	if _, err := os.Stat(expectedDest); err != nil {
		t.Fatalf("dest file not found: %v", err)
	}

	// Service จะลบต้นทางในขั้นหลังบันทึก DB สำเร็จ
	original, err := os.ReadFile(testFile)
	if err != nil || string(original) != "image-data" {
		t.Fatalf("source changed: %q, %v", original, err)
	}
}

func TestCopyImageDoesNotOverwrite(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	path := filepath.Join(source, "same.jpg")
	if err := os.WriteFile(path, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "same.jpg"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := CopyImage(ImageFile{Name: "same.jpg", Path: path, Size: 3}, destination)
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("expected duplicate-name error, got result=%+v error=%v", result, err)
	}
	data, err := os.ReadFile(filepath.Join(destination, "same.jpg"))
	if err != nil || string(data) != "old" {
		t.Fatalf("existing file changed: %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(destination, "same (1).jpg")); !os.IsNotExist(err) {
		t.Fatalf("duplicate should not create a renamed file: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "new" {
		t.Fatalf("duplicate should leave source unchanged: %q, %v", data, err)
	}
}
