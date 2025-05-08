package entutils

import (
	"context"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"fmt"
	"gitee.com/byx_darwin/go-tools/config/db"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"log"
	"time"
)

var (
	EventKey     = "mysql"
	QueryTextKey = attribute.Key("otel.mysql.query")
	ArgsTextKey  = attribute.Key("otel.mysql.args")
	DurationKey  = attribute.Key("otel.mysql.duration")
)

type EntOption interface {
	apply(driver *EntDriver)
}

type entOption func(driver *EntDriver)

func (fn entOption) apply(driver *EntDriver) {
	fn(driver)
}

type EntDriver struct {
	dialect.Driver                               // underlying driver.
	log            func(context.Context, ...any) // log function. defaults to log.Println.
	isTrace        bool                          // whether to trace queries.
}

func NewEntDriver(driver dialect.Driver,
	config *db.Config,
	options ...EntOption) *EntDriver {
	EventKey = config.Driver
	QueryTextKey = attribute.Key("otel." + EventKey + ".query")
	ArgsTextKey = attribute.Key("otel." + EventKey + ".args")
	DurationKey = attribute.Key("otel." + EventKey + ".duration")
	entDriver := DefaultMysqlDriver(driver)
	for _, option := range options {
		option.apply(entDriver)
	}
	return entDriver
}

func DefaultMysqlDriver(driver dialect.Driver) *EntDriver {
	return &EntDriver{
		Driver: driver,
		log: func(ctx context.Context, info ...any) {
			log.Println(info...)
		},
		isTrace: false,
	}
}
func WithLog(log func(context.Context, ...any)) EntOption {
	return entOption(func(driver *EntDriver) {
		if log != nil {
			driver.log = log
		}

	})
}

func WithTrace(isTrace bool) EntOption {
	return entOption(func(driver *EntDriver) {
		driver.isTrace = isTrace
	})
}

// Exec logs its params and calls the underlying driver Exec method.
func (d *EntDriver) Exec(ctx context.Context, query string, args, v any) error {
	start := time.Now()
	err := d.Driver.Exec(ctx, query, args, v)
	duration := time.Since(start)
	d.startTrace(ctx, query, duration, args)
	d.log(ctx, fmt.Sprintf("driver.Exec: query=%v args=%v time=%v", query, args, duration))
	return err
}

// ExecContext logs its params and calls the underlying driver ExecContext method if it is supported.
func (d *EntDriver) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	drv, ok := d.Driver.(interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
	})
	if !ok {
		return nil, fmt.Errorf("Driver.ExecContext is not supported")
	}
	start := time.Now()
	result, err := drv.ExecContext(ctx, query, args...)
	duration := time.Since(start)
	d.startTrace(ctx, query, duration, args)
	d.log(ctx, fmt.Sprintf("driver.ExecContext: query=%v args=%v time=%v", query, args, duration))
	return result, err
}

// Query logs its params and calls the underlying driver Query method.
func (d *EntDriver) Query(ctx context.Context, query string, args, v any) error {
	start := time.Now()
	err := d.Driver.Query(ctx, query, args, v)
	duration := time.Since(start)
	d.startTrace(ctx, query, duration, args)
	d.log(ctx, fmt.Sprintf("driver.Query: query=%v args=%v time=%v", query, args, duration))
	return err
}

// QueryContext logs its params and calls the underlying driver QueryContext method if it is supported.
func (d *EntDriver) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	drv, ok := d.Driver.(interface {
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	})
	if !ok {
		return nil, fmt.Errorf("Driver.QueryContext is not supported")
	}
	start := time.Now()
	rows, err := drv.QueryContext(ctx, query, args...)
	duration := time.Since(start)
	d.startTrace(ctx, query, duration, args)
	d.log(ctx, fmt.Sprintf("driver.QueryContext: query=%v args=%v time=%v", query, args, duration))
	return rows, err
}

func (d *EntDriver) startTrace(ctx context.Context,
	query string, duration time.Duration, args ...any) {
	if d.isTrace {
		span := trace.SpanFromContext(ctx)
		if span.IsRecording() {
			attrs := []attribute.KeyValue{
				QueryTextKey.String(query),
				ArgsTextKey.String(fmt.Sprint(args)),
				DurationKey.String(fmt.Sprintf("%v", duration)),
			}
			span.AddEvent(EventKey, trace.WithAttributes(attrs...))
		}
	}
}
