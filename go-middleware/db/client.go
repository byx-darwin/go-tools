package db

import (
	"context"
	"database/sql"
	"time"
)

// DB 数据库连接封装
type DB struct {
	*sql.DB
}

// NewDB 创建数据库连接并验证可达性。
// driver 为驱动名 (mysql, postgres, sqlite3)，source 为连接字符串。
func NewDB(ctx context.Context, driver, source string, cfg *Config) (*DB, func(), error) {
	database, err := sql.Open(driver, source)
	if err != nil {
		return nil, nil, err
	}

	// Apply pool config
	if cfg != nil {
		if cfg.MaxOpenCons > 0 {
			database.SetMaxOpenConns(cfg.MaxOpenCons)
		}
		if cfg.MaxIdleCons > 0 {
			database.SetMaxIdleConns(cfg.MaxIdleCons)
		}
		if cfg.ConMaxLifetime > 0 {
			database.SetConnMaxLifetime(time.Duration(cfg.ConMaxLifetime) * time.Second)
		}
		if cfg.MaxIdleTime > 0 {
			database.SetConnMaxIdleTime(time.Duration(cfg.MaxIdleTime) * time.Second)
		}
	}

	closeFn := func() { _ = database.Close() }

	if err := database.PingContext(ctx); err != nil {
		closeFn()
		return nil, nil, err
	}

	return &DB{DB: database}, closeFn, nil
}

// Ping 检查数据库连接是否存活。
func (db *DB) Ping(ctx context.Context) error {
	return db.PingContext(ctx)
}
