package analysis

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestSHA256FileSameContentHasSameHash(t *testing.T) {
	directory := t.TempDir()
	content := []byte("same image content")
	firstPath := filepath.Join(directory, "first.jpg")
	secondPath := filepath.Join(directory, "second.jpg")

	if err := os.WriteFile(firstPath, content, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, content, 0600); err != nil {
		t.Fatal(err)
	}

	firstHash, err := SHA256File(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	secondHash, err := SHA256File(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	if firstHash != secondHash {
		t.Fatalf("same content produced different hashes: %q != %q", firstHash, secondHash)
	}
}

func TestSHA256FileDifferentContentHasDifferentHash(t *testing.T) {
	directory := t.TempDir()
	firstPath := filepath.Join(directory, "first.jpg")
	secondPath := filepath.Join(directory, "second.jpg")

	if err := os.WriteFile(firstPath, []byte("image content A"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, []byte("image content B"), 0600); err != nil {
		t.Fatal(err)
	}

	firstHash, err := SHA256File(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	secondHash, err := SHA256File(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	if firstHash == secondHash {
		t.Fatalf("different content produced the same hash: %q", firstHash)
	}
}

func TestSHA256FileSameNameDifferentContentHasDifferentHash(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "photo.jpg")

	if err := os.WriteFile(path, []byte("old photo content"), 0600); err != nil {
		t.Fatal(err)
	}
	oldHash, err := SHA256File(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("new photo content"), 0600); err != nil {
		t.Fatal(err)
	}
	newHash, err := SHA256File(path)
	if err != nil {
		t.Fatal(err)
	}
	if oldHash == newHash {
		t.Fatalf("same filename with different content kept the same hash: %q", oldHash)
	}
}

func TestSHA256FileLargeFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "large.bin")

	content := make([]byte, 3*hashBufferSize+17)
	for index := range content {
		content[index] = byte((index*31 + 7) % 256)
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	expectedDigest := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(expectedDigest[:])
	actualHash, err := SHA256File(path)
	if err != nil {
		t.Fatal(err)
	}
	if actualHash != expectedHash {
		t.Fatalf("large file hash=%q, want %q", actualHash, expectedHash)
	}
}

func TestSHA256FileReturnsErrorWhenPathCannotBeRead(t *testing.T) {
	directory := t.TempDir()

	if _, err := SHA256File(filepath.Join(directory, "missing.jpg")); err == nil {
		t.Fatal("expected an error for a missing file")
	}

	notAFile := filepath.Join(directory, "directory.jpg")
	if err := os.Mkdir(notAFile, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := SHA256File(notAFile); err == nil {
		t.Fatal("expected an error when reading a directory as a file")
	}
}
