package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

func EnvPaths(workingDir, executable, configDir string) []string {
	paths := []string{filepath.Join(workingDir, ".env"), filepath.Join(filepath.Dir(executable), ".env")}
	for dir := filepath.Dir(executable); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if strings.HasSuffix(strings.ToLower(dir), ".app") {
			paths = append(paths, filepath.Join(filepath.Dir(dir), ".env"))
			break
		}
	}
	if configDir != "" {
		paths = append(paths, filepath.Join(configDir, "PhotoBackup", ".env"))
	}
	return paths
}

func LoadEnv() error {
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	configDir, _ := os.UserConfigDir()
	for _, path := range EnvPaths(workingDir, executable, configDir) {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		return godotenv.Load(path)
	}
	return nil
}
