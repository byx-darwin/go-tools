package clickhouse

import (
	"context"
	"errors"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// fakeRow 是 driver.Row 的最小实现，仅用于测试 tracedConn 的透传行为。
type fakeRow struct{ err error }

func (r fakeRow) Err() error           { return r.err }
func (r fakeRow) Scan(...any) error    { return r.err }
func (r fakeRow) ScanStruct(any) error { return r.err }

// fakeConn 是 driver.Conn 的最小实现，记录调用参数并返回预设结果，
// 用于验证 tracedConn 是否正确透传到内层 Conn。
type fakeConn struct {
	err          error
	lastQuery    string
	selectCalled bool
	queryCalled  bool
	rowCalled    bool
	execCalled   bool
	batchCalled  bool
	pingCalled   bool
}

func (c *fakeConn) Contributors() []string                        { return nil }
func (c *fakeConn) ServerVersion() (*driver.ServerVersion, error) { return nil, nil }
func (c *fakeConn) Stats() driver.Stats                           { return driver.Stats{} }
func (c *fakeConn) Close() error                                  { return nil }

func (c *fakeConn) Select(_ context.Context, _ any, query string, _ ...any) error {
	c.selectCalled = true
	c.lastQuery = query
	return c.err
}

func (c *fakeConn) Query(_ context.Context, query string, _ ...any) (driver.Rows, error) {
	c.queryCalled = true
	c.lastQuery = query
	if c.err != nil {
		return nil, c.err
	}
	return nil, nil
}

func (c *fakeConn) QueryRow(_ context.Context, query string, _ ...any) driver.Row {
	c.rowCalled = true
	c.lastQuery = query
	return fakeRow{err: c.err}
}

func (c *fakeConn) Exec(_ context.Context, query string, _ ...any) error {
	c.execCalled = true
	c.lastQuery = query
	return c.err
}

func (c *fakeConn) AsyncInsert(_ context.Context, query string, _ bool, _ ...any) error {
	c.lastQuery = query
	return c.err
}

func (c *fakeConn) PrepareBatch(_ context.Context, query string, _ ...driver.PrepareBatchOption) (driver.Batch, error) {
	c.batchCalled = true
	c.lastQuery = query
	if c.err != nil {
		return nil, c.err
	}
	return nil, nil
}

func (c *fakeConn) Ping(_ context.Context) error {
	c.pingCalled = true
	return c.err
}

func TestTracedConn_PassesThroughUnwrappedMethods(t *testing.T) {
	inner := &fakeConn{}
	conn := newTracedConn(inner)

	assert.NoError(t, conn.Close())
	assert.Equal(t, driver.Stats{}, conn.Stats())
}

func TestTracedConn_Success(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	inner := &fakeConn{}
	conn := &tracedConn{Conn: inner, tracer: tp.Tracer(instrumentationName)}
	ctx := context.Background()

	require.NoError(t, conn.Select(ctx, nil, "SELECT 1"))
	assert.True(t, inner.selectCalled)

	_, err := conn.Query(ctx, "SELECT 2")
	require.NoError(t, err)
	assert.True(t, inner.queryCalled)

	row := conn.QueryRow(ctx, "SELECT 3")
	require.NoError(t, row.Err())
	assert.True(t, inner.rowCalled)

	require.NoError(t, conn.Exec(ctx, "INSERT INTO t VALUES (1)"))
	assert.True(t, inner.execCalled)

	_, err = conn.PrepareBatch(ctx, "INSERT INTO t")
	require.NoError(t, err)
	assert.True(t, inner.batchCalled)

	require.NoError(t, conn.Ping(ctx))
	assert.True(t, inner.pingCalled)

	spans := exporter.GetSpans()
	require.Len(t, spans, 6)
	names := make([]string, 0, len(spans))
	for _, s := range spans {
		names = append(names, s.Name)
		// endSpan 只在出错时显式 SetStatus，成功路径保持默认 Unset，
		// 不覆盖上层调用者可能已设置的状态。
		assert.Equal(t, codes.Unset, s.Status.Code)
	}
	assert.ElementsMatch(t, []string{
		"clickhouse.Select", "clickhouse.Query", "clickhouse.QueryRow",
		"clickhouse.Exec", "clickhouse.PrepareBatch", "clickhouse.Ping",
	}, names)
}

func TestTracedConn_Failure(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	wantErr := errors.New("boom")
	inner := &fakeConn{err: wantErr}
	conn := &tracedConn{Conn: inner, tracer: tp.Tracer(instrumentationName)}
	ctx := context.Background()

	assert.Equal(t, wantErr, conn.Select(ctx, nil, "SELECT 1"))
	assert.Equal(t, wantErr, conn.Exec(ctx, "INSERT INTO t VALUES (1)"))

	spans := exporter.GetSpans()
	require.Len(t, spans, 2)
	for _, s := range spans {
		assert.Equal(t, codes.Error, s.Status.Code)
		require.Len(t, s.Events, 1)
		assert.Equal(t, "exception", s.Events[0].Name)
	}
}
