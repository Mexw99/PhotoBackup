package main

import (
	"fmt"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"photo-backup/backup"
	"photo-backup/service"
)

func (a *App) selectFolder(title string) (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("application is not ready")
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: title})
}

func (a *App) SelectSourceFolder() (string, error) {
	return a.selectFolder("เลือกโฟลเดอร์รูปภาพต้นทาง")
}

func (a *App) SelectDestFolder() (string, error) {
	return a.selectFolder("เลือกโฟลเดอร์สำหรับเก็บรูปภาพ Backup")
}

func (a *App) ListImageFiles(sourcePath string) ([]backup.ImageFile, error) {
	return service.ListImageFiles(sourcePath)
}

func (a *App) GetBackupFolder() string { return service.DefaultBackupFolder() }

func (a *App) StartBackup(sourcePath, destPath string) (backup.BackupSummary, error) {
	if a.service == nil {
		return backup.BackupSummary{}, fmt.Errorf("database is not connected")
	}
	return a.service.StartBackup(sourcePath, destPath, func(current, total int, name string) {
		runtime.EventsEmit(a.ctx, "backup:progress", map[string]interface{}{
			"current": current, "total": total, "name": name,
		})
	})
}
