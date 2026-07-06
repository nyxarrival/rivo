package database

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Open(driver string, dsn string) (*gorm.DB, error) {
	driver = normalizeDriver(driver)
	dialector, err := dialectorFor(driver, dsn)
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	if driver == "sqlite" {
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
	} else {
		sqlDB.SetMaxOpenConns(50)
		sqlDB.SetMaxIdleConns(10)
	}
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}

func normalizeDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", "mysql":
		return "mysql"
	case "sqlite", "sqlite3":
		return "sqlite"
	default:
		return strings.ToLower(strings.TrimSpace(driver))
	}
}

func dialectorFor(driver string, dsn string) (gorm.Dialector, error) {
	switch driver {
	case "mysql":
		if strings.TrimSpace(dsn) == "" {
			return nil, fmt.Errorf("mysql dsn is required")
		}
		return mysql.Open(dsn), nil
	case "sqlite":
		if strings.TrimSpace(dsn) == "" {
			dsn = "data/rivo.db"
		}
		if err := ensureSQLiteDir(dsn); err != nil {
			return nil, err
		}
		return sqlite.Open(dsn), nil
	default:
		return nil, fmt.Errorf("unsupported database driver %q", driver)
	}
}

func ensureSQLiteDir(dsn string) error {
	if strings.HasPrefix(dsn, "file:") || dsn == ":memory:" {
		return nil
	}
	dir := filepath.Dir(dsn)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
