package config

import (
	"path/filepath"
	"testing"
)

func TestEnvPathsSupportDesktopLaunch(t *testing.T) {
	root := t.TempDir()
	for _, executable := range []string{
		filepath.Join(root, "photo-backup.exe"),
		filepath.Join(root, "photo-backup.app", "Contents", "MacOS", "photo-backup"),
	} {
		paths := EnvPaths(filepath.Join(root, "working"), executable, filepath.Join(root, "settings"))
		found := false
		for _, path := range paths {
			if path == filepath.Join(root, ".env") {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing env next to distributed app: %v", paths)
		}
		if paths[0] != filepath.Join(root, "working", ".env") {
			t.Fatal("working directory must take precedence")
		}
		if paths[len(paths)-1] != filepath.Join(root, "settings", "PhotoBackup", ".env") {
			t.Fatal("missing config fallback")
		}
	}
}
