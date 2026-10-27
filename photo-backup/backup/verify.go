package backup

import (
	"bytes"
	"errors"
	"io"
	"os"
)

type IntegrityResult struct {
	Filename     string `json:"filename"`
	Path         string `json:"path"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	ExpectedSize int64  `json:"expectedSize"`
	ActualSize   int64  `json:"actualSize"`
}

func EqualFiles(sourcePath, destPath string) (bool, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return false, err
	}
	defer source.Close()
	destination, err := os.Open(destPath)
	if err != nil {
		return false, err
	}
	defer destination.Close()

	var sourceData, destData [64 * 1024]byte
	for {
		sourceSize, sourceErr := io.ReadFull(source, sourceData[:])
		destSize, destErr := io.ReadFull(destination, destData[:])
		if sourceErr != nil && !errors.Is(sourceErr, io.EOF) && !errors.Is(sourceErr, io.ErrUnexpectedEOF) {
			return false, sourceErr
		}
		if destErr != nil && !errors.Is(destErr, io.EOF) && !errors.Is(destErr, io.ErrUnexpectedEOF) {
			return false, destErr
		}
		if sourceSize != destSize || !bytes.Equal(sourceData[:sourceSize], destData[:destSize]) {
			return false, nil
		}
		if sourceErr != nil || destErr != nil {
			return true, nil
		}
	}
}
