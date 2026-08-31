package db

import (
	"context"
	"database/sql"

	"github.com/XSAM/otelsql"
	"go.opentelemetry.io/otel/metric"
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
//	    db.WithTrace(),
//	)
func NewDB(ctx context.Context, opts ...Option) (*DB, func(), error) {
	cfg := &dbConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	var (
		database *sql.DB
		err      error
	)
	if cfg.trace {
		database, err = otelsql.Open(cfg.driver, cfg.source)
	} else {
		database, err = sql.Open(cfg.driver, cfg.source)
	}
	if err != nil {
		return nil, nil, ErrOpen.Wrap(err)
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
			database.SetConnMaxLifetime(cfg.pool.ConMaxLifetime)
		}
		if cfg.pool.MaxIdleTime > 0 {
			database.SetConnMaxIdleTime(cfg.pool.MaxIdleTime)
		}
	}

	var statsReg metric.Registration
	if cfg.trace {
		// 连接池指标注册失败不应阻断数据库连接建立，仅静默降级。
		statsReg, _ = otelsql.RegisterDBStatsMetrics(database)
	}

	closeFn := func() {
		if statsReg != nil {
			_ = statsReg.Unregister()
		}
		_ = database.Close()
	}

	if err := database.PingContext(ctx); err != nil {
		closeFn()
		return nil, nil, ErrConnect.Wrap(err)
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
