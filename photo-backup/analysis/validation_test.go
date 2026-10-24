package analysis

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateImageAcceptsReadablePNG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "photo.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	picture := image.NewRGBA(image.Rect(0, 0, 2, 3))
	picture.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(file, picture); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ValidateImage(path); err != nil {
		t.Fatalf("valid PNG rejected: %v", err)
	}
}

func TestValidateImageAcceptsReadableJPEG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "photo.jpg")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(file, image.NewRGBA(image.Rect(0, 0, 3, 2)), nil); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ValidateImage(path); err != nil {
		t.Fatalf("valid JPEG rejected: %v", err)
	}
}

func TestValidateImageRejectsMissingAndInvalidFiles(t *testing.T) {
	folder := t.TempDir()
	if err := ValidateImage(filepath.Join(folder, "missing.png")); err == nil {
		t.Fatal("missing file was accepted")
	}
	invalid := filepath.Join(folder, "broken.png")
	if err := os.WriteFile(invalid, []byte("not an image"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateImage(invalid); err == nil {
		t.Fatal("invalid image was accepted")
	}
	empty := filepath.Join(folder, "empty.jpg")
	if err := os.WriteFile(empty, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateImage(empty); err == nil {
		t.Fatal("empty image was accepted")
	}
	unsupported := filepath.Join(folder, "notes.txt")
	if err := os.WriteFile(unsupported, []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateImage(unsupported); err == nil {
		t.Fatal("unsupported extension was accepted")
	}
}
