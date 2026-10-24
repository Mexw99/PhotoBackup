package analysis

import (
	"bytes"
	"crypto/sha256"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestCloudPreprocessingResizesInMemoryWithoutChangingSourceHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.png")
	source := image.NewNRGBA(image.Rect(0, 0, 2400, 1200))
	for y := 0; y < source.Bounds().Dy(); y += 50 {
		for x := 0; x < source.Bounds().Dx(); x += 50 {
			source.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	if err := writePNG(path, source); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	expected := sha256.Sum256(original)
	processed, mimeType, err := readImageForCloudAI(path)
	if err != nil || mimeType != "image/png" {
		t.Fatalf("preprocess: mime=%q err=%v", mimeType, err)
	}
	decoded, _, err := image.Decode(bytes.NewReader(processed))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() > cloudMaxImageSide || decoded.Bounds().Dy() > cloudMaxImageSide {
		t.Fatalf("processed size too large: %v", decoded.Bounds())
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, after) {
		t.Fatal("cloud preprocessing modified the source file")
	}
	actual := sha256.Sum256(after)
	if actual != expected {
		t.Fatal("cloud preprocessing changed the source hash")
	}
}

func TestCloudPreprocessingKeepsSmallPNGBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "small.png")
	source := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	if err := writePNG(path, source); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	processed, _, err := readImageForCloudAI(path)
	if err != nil || !bytes.Equal(original, processed) {
		t.Fatalf("small image was unexpectedly re-encoded: err=%v", err)
	}
}

func writePNG(path string, source image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(file, source); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
