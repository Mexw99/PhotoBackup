package backup

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEqualFilesChecksAllBytesAndLengths(t *testing.T) {
	data := bytes.Repeat([]byte("a"), 128*1024+17)
	changed := append([]byte(nil), data...)
	changed[len(changed)-1] = 'b'
	for _, test := range []struct {
		name        string
		source      []byte
		destination []byte
		equal       bool
	}{
		{name: "multiple chunks", source: data, destination: data, equal: true},
		{name: "exact chunk", source: data[:64*1024], destination: data[:64*1024], equal: true},
		{name: "same size changed last byte", source: data, destination: changed},
		{name: "shorter destination", source: data, destination: data[:len(data)-1]},
		{name: "empty files", equal: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			folder := t.TempDir()
			source, destination := filepath.Join(folder, "source"), filepath.Join(folder, "destination")
			if err := os.WriteFile(source, test.source, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(destination, test.destination, 0600); err != nil {
				t.Fatal(err)
			}
			equal, err := EqualFiles(source, destination)
			if err != nil || equal != test.equal {
				t.Fatalf("equal=%v, want %v, error=%v", equal, test.equal, err)
			}
		})
	}
}

func TestCopyImageFailureKeepsSourceAndCleansDestination(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	path := filepath.Join(source, "photo.jpg")
	if err := os.WriteFile(path, []byte("image bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	// ขนาดไม่ตรงกับข้อมูลที่สแกนไว้ ต้องหยุดโดยไม่ลบต้นทาง
	if _, err := CopyImage(ImageFile{Name: "photo.jpg", Path: path, Size: 100}, destination); err == nil {
		t.Fatal("expected changed source size to fail")
	}
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != "image bytes" {
		t.Fatalf("source changed on failure: %q, %v", actual, err)
	}
	files, err := os.ReadDir(destination)
	if err != nil || len(files) != 0 {
		t.Fatalf("incomplete destination remains: %+v, %v", files, err)
	}
}
