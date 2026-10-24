package analysis

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

const hashBufferSize = 64 * 1024

// SHA256File calculates the SHA-256 digest of a file by streaming it in
// fixed-size chunks. It does not load the complete file into memory.
func SHA256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	digest := sha256.New()
	buffer := make([]byte, hashBufferSize)
	if _, err := io.CopyBuffer(digest, file, buffer); err != nil {
		return "", err
	}

	return hex.EncodeToString(digest.Sum(nil)), nil
}
