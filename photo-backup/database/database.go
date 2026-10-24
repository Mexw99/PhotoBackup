package database

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"photo-backup/model"
)

type Database struct {
	conn *gorm.DB
}

func DefaultPath() (string, error) {
	// เก็บฐานข้อมูลไว้ในโฟลเดอร์ที่ใช้เปิดโปรแกรม เพื่อให้หาไฟล์ได้ง่าย
	// และใช้ได้ทั้ง Windows/macOS โดยไม่ผูกกับชื่อ User หรือ Drive ใด ๆ
	baseDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return filepath.Join(baseDir, "photo-backup.db"), nil
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

	// go-sqlite (the driver used by glebarez/sqlite) supports these SQLite
	// pragmas. Apply them after opening the plain Windows/macOS path so a
	// platform-specific path is never parsed as a DSN query string.
	conn, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	// PRAGMA values are connection-scoped. Use one pooled connection so the
	// values applied below cannot be lost on a second connection.
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
	if err := conn.Exec("PRAGMA journal_mode = WAL").Error; err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	if err := conn.SetupJoinTable(&model.Photo{}, "Tags", &model.PhotoTag{}); err != nil {
		return nil, err
	}

	if err := conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(
			&model.Photo{},
			&model.Tag{},
			&model.PhotoTag{},
			&model.FileRecord{},
			&model.Setting{},
			&model.AIResult{},
			&model.AIJob{},
			&model.AIUsage{},
		); err != nil {
			return err
		}
		// Older databases already have photo_tags from GORM's implicit join
		// table. Add only the new nullable-safe source column; old links are
		// deliberately classified as user-created data.
		if !tx.Migrator().HasColumn(&model.PhotoTag{}, "source") {
			if err := tx.Migrator().AddColumn(&model.PhotoTag{}, "Source"); err != nil {
				return err
			}
		}
		if err := migratePhotoTagPrimaryKey(tx); err != nil {
			return err
		}
		return tx.Exec("UPDATE photo_tags SET source = 'user' WHERE source IS NULL OR source = ''").Error
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

// migratePhotoTagPrimaryKey upgrades the implicit GORM join table created by
// older versions. Those versions used (photo_id, tag_id) as the key, which
// cannot represent the same named tag as both user and AI provenance.
func migratePhotoTagPrimaryKey(tx *gorm.DB) error {
	rows, err := tx.Raw("PRAGMA table_info(photo_tags)").Rows()
	if err != nil {
		return err
	}
	defer rows.Close()
	var sourcePK, photoPK, tagPK int
	for rows.Next() {
		var cid, pk int
		var name, columnType string
		var notNull int
		var defaultValue interface{}
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		switch name {
		case "photo_id":
			photoPK = pk
		case "tag_id":
			tagPK = pk
		case "source":
			sourcePK = pk
		}
	}
	if sourcePK != 0 || photoPK == 0 || tagPK == 0 {
		return nil
	}
	if err := tx.Exec("ALTER TABLE photo_tags RENAME TO photo_tags_legacy").Error; err != nil {
		return err
	}
	if err := tx.Exec(`CREATE TABLE photo_tags (
		photo_id integer NOT NULL,
		tag_id integer NOT NULL,
		source text NOT NULL DEFAULT 'user',
		PRIMARY KEY (photo_id, tag_id, source),
		CONSTRAINT fk_photo_tags_photo FOREIGN KEY (photo_id) REFERENCES photos(id),
		CONSTRAINT fk_photo_tags_tag FOREIGN KEY (tag_id) REFERENCES tags(id)
	)`).Error; err != nil {
		return err
	}
	if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_photo_tags_source ON photo_tags(source)").Error; err != nil {
		return err
	}
	if err := tx.Exec("INSERT INTO photo_tags (photo_id, tag_id, source) SELECT photo_id, tag_id, 'user' FROM photo_tags_legacy").Error; err != nil {
		return err
	}
	return tx.Exec("DROP TABLE photo_tags_legacy").Error
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
