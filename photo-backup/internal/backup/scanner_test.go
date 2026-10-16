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

func TestCopyFilesMovesAndRemovesSource(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	testFile := filepath.Join(srcDir, "pic.jpg")
	if err := os.WriteFile(testFile, []byte("image-data"), 0644); err != nil {
		t.Fatal(err)
	}

	images := []ImageFile{
		{Name: "pic.jpg", Path: testFile, Size: 10},
	}

	summary := CopyFiles(images, destDir, nil)
	if summary.Success != 1 {
		t.Fatalf("expected 1 success, got %d (failed: %d)", summary.Success, summary.Failed)
	}

	// 1. ตรวจสอบว่าไฟล์ปลายทางมีอยู่จริง
	expectedDest := filepath.Join(destDir, "pic.jpg")
	if _, err := os.Stat(expectedDest); err != nil {
		t.Fatalf("dest file not found: %v", err)
	}

	// 2. ตรวจสอบว่าไฟล์ต้นทางถูกลบออกแล้ว (ย้ายสำเร็จ)
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Fatalf("source file should have been deleted, but still exists")
	}
}
