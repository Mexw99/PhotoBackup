package service_test

import (
	"path/filepath"
	"testing"
)

func TestBackupSettingsValidateFoldersAndPersist(t *testing.T) {
	s := newService(t)
	source, destination := t.TempDir(), filepath.Join(t.TempDir(), "backup")
	if err := s.SaveBackupSettings(source, destination); err != nil {
		t.Fatal(err)
	}
	setting, err := s.GetBackupSettings()
	if err != nil || setting.Source != source || setting.Dest != destination {
		t.Fatalf("saved settings: %+v, %v", setting, err)
	}
	for _, invalid := range []string{source, filepath.Join(source, "inside-source")} {
		if err := s.SaveBackupSettings(source, invalid); err == nil {
			t.Fatalf("allowed destination %q inside source", invalid)
		}
	}
	setting, err = s.GetBackupSettings()
	if err != nil || setting.Dest != destination {
		t.Fatalf("invalid folders replaced saved settings: %+v, %v", setting, err)
	}
}
