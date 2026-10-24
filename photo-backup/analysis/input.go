package analysis

import (
	"os"
	"path/filepath"
	"strings"
)

func readImageForAI(path string) ([]byte, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	return data, imageMIME(path), nil
}

func imageMIME(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	default:
		return "image/jpeg"
	}
}
