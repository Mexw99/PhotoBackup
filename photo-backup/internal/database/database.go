package database

import (
	"errors"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"photo-backup/internal/models"
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

	if err := conn.AutoMigrate(
		&models.Photo{},
		&models.Tag{},
	); err != nil {
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
