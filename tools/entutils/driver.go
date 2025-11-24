package entutils

import (
	"context"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"gitee.com/byx_darwin/go-tools/config/db"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/uptrace/opentelemetry-go-extra/otelsql"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
)

func NewDriver(ctx context.Context,
	config *db.Config, isTrace bool, log hlog.CtxLogger) (*EntDriver, error) {
	sql, err := otelsql.Open(config.Driver, config.Source)
	if err != nil {
		log.CtxErrorf(ctx, "failed opening connection to sql: %v", err)
		return nil, err
	}
	sql.SetMaxIdleConns(config.MaxIdleCons)
	sql.SetMaxOpenConns(config.MaxOpenCons)
	sql.SetConnMaxIdleTime(time.Duration(config.MaxIdleTime) * time.Second)
	sql.SetConnMaxIdleTime(time.Duration(config.ConMaxLifetime) * time.Second)
	attribute := semconv.DBSystemSqlite
	switch config.Driver {
	case "postgres":
		attribute = semconv.DBSystemPostgreSQL
	case "sqlite3":
		attribute = semconv.DBSystemSqlite
	case "mysql":
		attribute = semconv.DBSystemMySQL
	case "mongodb":
		attribute = semconv.DBSystemMongoDB

	}
	otelsql.WithAttributes(attribute)
	otelsql.WithDBName(config.Name)
	otelsql.WithDBSystem(config.Driver)
	var drv dialect.Driver
	drv = entsql.OpenDB(config.Driver, sql)
	opts := make([]EntOption, 0, 2)
	opts = append(opts, WithTrace(isTrace))
	opts = append(opts, WithLog(func(ctx context.Context, info ...any) {
		if config.SqlLog {
			log.CtxDebugf(ctx, "%v", info)
		}
	}))
	entMysqlDriver := NewEntDriver(drv, config, opts...)
	return entMysqlDriver, nil
}
