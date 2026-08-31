package clickhouse

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// instrumentationName OTel instrumentation 标识。
const instrumentationName = "github.com/byx-darwin/go-tools/go-middleware/clickhouse"

// tracedConn 给 driver.Conn 的查询类方法包一层 OTel span。
//
// 未覆盖的方法（Contributors/ServerVersion/Stats/Close/AsyncInsert）通过接口
// 嵌入直接透传给内层 Conn。
type tracedConn struct {
	driver.Conn
	tracer trace.Tracer
}

// newTracedConn 用全局 TracerProvider 包一层 conn，与 db/es/redis 的 WithTrace 一致，
// 不引入 go-middleware 自己的 Provider 管理。
func newTracedConn(conn driver.Conn) driver.Conn {
	return &tracedConn{Conn: conn, tracer: otel.Tracer(instrumentationName)}
}

// startSpan 开启一个 span，并把 SpanContext 透传给 clickhouse-go 原生的
// clickhouse.WithSpan，使服务端 system.opentelemetry_span_log 也能关联上。
func (c *tracedConn) startSpan(ctx context.Context, op, query string) (context.Context, trace.Span) {
	ctx, span := c.tracer.Start(ctx, "clickhouse."+op, trace.WithAttributes(
		attribute.String("db.system", "clickhouse"),
		attribute.String("db.statement", query),
	))
	ctx = clickhouse.Context(ctx, clickhouse.WithSpan(span.SpanContext()))
	return ctx, span
}

func endSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

// Select 追踪版 Select。
func (c *tracedConn) Select(ctx context.Context, dest any, query string, args ...any) error {
	ctx, span := c.startSpan(ctx, "Select", query)
	err := c.Conn.Select(ctx, dest, query, args...)
	endSpan(span, err)
	return err
}

// Query 追踪版 Query。
func (c *tracedConn) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	ctx, span := c.startSpan(ctx, "Query", query)
	rows, err := c.Conn.Query(ctx, query, args...)
	endSpan(span, err)
	return rows, err
}

// QueryRow 追踪版 QueryRow。
//
// driver.Row 没有独立的 error 返回值，调用后立即读取 Err() 记录到 span——
// 原生协议下 QueryRow 是同步执行的，此时错误状态已经确定。
func (c *tracedConn) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	ctx, span := c.startSpan(ctx, "QueryRow", query)
	row := c.Conn.QueryRow(ctx, query, args...)
	endSpan(span, row.Err())
	return row
}

// Exec 追踪版 Exec。
func (c *tracedConn) Exec(ctx context.Context, query string, args ...any) error {
	ctx, span := c.startSpan(ctx, "Exec", query)
	err := c.Conn.Exec(ctx, query, args...)
	endSpan(span, err)
	return err
}

// PrepareBatch 追踪版 PrepareBatch。
//
// 只覆盖 PrepareBatch 本身的协商阶段：driver.Batch.Send() 不接收 context 参数，
// 无法把调用方的 trace 上下文传下去，包一层只会产生脱离调用链的孤立 span，
// 价值有限，故不再包装返回的 Batch。
func (c *tracedConn) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	ctx, span := c.startSpan(ctx, "PrepareBatch", query)
	batch, err := c.Conn.PrepareBatch(ctx, query, opts...)
	endSpan(span, err)
	return batch, err
}

// Ping 追踪版 Ping。
func (c *tracedConn) Ping(ctx context.Context) error {
	ctx, span := c.startSpan(ctx, "Ping", "")
	err := c.Conn.Ping(ctx)
	endSpan(span, err)
	return err
}
