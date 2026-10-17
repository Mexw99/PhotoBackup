package analysis

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectColor(t *testing.T) {
	for _, test := range []struct {
		name  string
		pixel color.NRGBA
	}{
		{"สีแดง", color.NRGBA{R: 220, G: 40, B: 40, A: 255}},
		{"สีเขียว", color.NRGBA{R: 35, G: 180, B: 60, A: 255}},
		{"สีน้ำเงิน", color.NRGBA{R: 30, G: 70, B: 220, A: 255}},
		{"สีเหลือง", color.NRGBA{R: 255, G: 255, A: 255}},
		{"สีขาว", color.NRGBA{R: 255, G: 255, B: 255, A: 255}},
		{"สีดำ", color.NRGBA{A: 255}},
	} {
		t.Run(test.name, func(t *testing.T) {
			picture := image.NewNRGBA(image.Rect(0, 0, 8, 8))
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					picture.Set(x, y, test.pixel)
				}
			}
			path := filepath.Join(t.TempDir(), "color.png")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(file, picture); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			got, err := DetectColor(path)
			if err != nil || got != test.name {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}
