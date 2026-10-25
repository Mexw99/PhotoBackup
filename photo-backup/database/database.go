package database

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"photo-backup/model"
)

const appDirectoryName = "PhotoBackup"

type Database struct {
	conn *gorm.DB
}

func DefaultPath() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(
		baseDir,
		appDirectoryName,
		"photo-backup.db",
	), nil
}

func OpenDefault() (*Database, string, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, "", err
	}

	db, err := OpenAt(path)
	if err != nil {
		return nil, path, err
	}

	return db, path, nil
}

func OpenAt(path string) (*Database, error) {
	if path == "" {
		return nil, errors.New("database path is empty")
	}

	directory := filepath.Dir(path)

	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, err
	}

	conn, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	// SQLite permits one writer at a time. A single connection plus a short
	// busy timeout prevents parallel backup workers from failing with SQLITE_BUSY.
	sqlDB, err := conn.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if err := conn.Exec("PRAGMA busy_timeout = 5000").Error; err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	if err := conn.SetupJoinTable(&model.Photo{}, "Tags", &model.PhotoTag{}); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	if err := conn.SetupJoinTable(&model.Tag{}, "Photos", &model.PhotoTag{}); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	if err := conn.Transaction(func(tx *gorm.DB) error {
		if tx.Migrator().HasColumn(&model.Photo{}, "color_tag") {
			// ColorTag เป็นระบบวิเคราะห์แบบเดิมที่เลิกใช้แล้ว จึงเอาออกจาก schema
			// โดยคงข้อมูลรูป, Tag ที่ผู้ใช้เพิ่มเอง และตารางประวัติเอาไว้
			if err := tx.Exec("ALTER TABLE photos DROP COLUMN color_tag").Error; err != nil {
				return err
			}
		}
		return tx.AutoMigrate(
			&model.Photo{},
			&model.Tag{},
			&model.PhotoTag{},
			&model.FileRecord{},
			&model.Setting{},
		)
	}); err != nil {
		if sqlDB, closeErr := conn.DB(); closeErr == nil {
			_ = sqlDB.Close()
		}
		return nil, err
	}

	return &Database{
		conn: conn,
	}, nil
}

func (d *Database) GORM() *gorm.DB {
	if d == nil {
		return nil
	}

	return d.conn
}

func (d *Database) Close() error {
	if d == nil || d.conn == nil {
		return nil
	}

	sqlDB, err := d.conn.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}
