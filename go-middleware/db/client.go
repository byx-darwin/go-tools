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

// NewDB 创建数据库连接并验证可达性，支持 Options 配置。
//
// 默认配置：无连接池限制。
//
// 用法：
//
//	db, cleanup, err := db.NewDB(ctx,
//	    db.WithDriver("mysql"),
//	    db.WithSource("user:pass@tcp(localhost:3306)/dbname"),
//	    db.WithPoolConfig(&db.Config{MaxOpenCons: 10}),
//	)
func NewDB(ctx context.Context, opts ...Option) (*DB, func(), error) {
	cfg := &dbConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	database, err := sql.Open(cfg.driver, cfg.source)
	if err != nil {
		return nil, nil, err
	}

	// Apply pool config
	if cfg.pool != nil {
		if cfg.pool.MaxOpenCons > 0 {
			database.SetMaxOpenConns(cfg.pool.MaxOpenCons)
		}
		if cfg.pool.MaxIdleCons > 0 {
			database.SetMaxIdleConns(cfg.pool.MaxIdleCons)
		}
		if cfg.pool.ConMaxLifetime > 0 {
			database.SetConnMaxLifetime(time.Duration(cfg.pool.ConMaxLifetime) * time.Second)
		}
		if cfg.pool.MaxIdleTime > 0 {
			database.SetConnMaxIdleTime(time.Duration(cfg.pool.MaxIdleTime) * time.Second)
		}
	}

	closeFn := func() { _ = database.Close() }

	if err := database.PingContext(ctx); err != nil {
		closeFn()
		return nil, nil, err
	}

	return &DB{DB: database}, closeFn, nil
}

// NewDBLegacy 创建数据库连接并验证可达性。
//
// Deprecated: 使用 NewDB 配合 Options 替代。
func NewDBLegacy(ctx context.Context, driver, source string, cfg *Config) (*DB, func(), error) {
	return NewDB(ctx, WithDriver(driver), WithSource(source), WithPoolConfig(cfg))
}

// Ping 检查数据库连接是否存活。
func (db *DB) Ping(ctx context.Context) error {
	return db.PingContext(ctx)
}
