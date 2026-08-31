package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeConn 是一个最小可用的 database/sql/driver.Conn 实现，仅用于测试
// NewDB 的连接建立/Ping/Close 流程，不需要真实数据库。
type fakeConn struct{}

func (fakeConn) Prepare(_ string) (driver.Stmt, error) { return nil, errors.New("not implemented") }
func (fakeConn) Close() error                          { return nil }
func (fakeConn) Begin() (driver.Tx, error)             { return nil, errors.New("not implemented") } //nolint:staticcheck // 测试用最小实现，仅需满足 driver.Conn 接口

// fakeDriver 每次 Open 都返回一个可用的 fakeConn。
type fakeDriver struct{}

func (fakeDriver) Open(_ string) (driver.Conn, error) { return fakeConn{}, nil }

var registerFakeDriverOnce sync.Once

func registerFakeDriver(t *testing.T) string {
	t.Helper()
	const name = "db_fake_driver"
	registerFakeDriverOnce.Do(func() {
		sql.Register(name, fakeDriver{})
	})
	return name
}

func TestWithTrace(t *testing.T) {
	c := &dbConfig{}
	WithTrace()(c)
	assert.True(t, c.trace)
}

func TestNewDB_Basic(t *testing.T) {
	driverName := registerFakeDriver(t)

	database, cleanup, err := NewDB(context.Background(),
		WithDriver(driverName),
		WithSource("fake"),
	)
	require.NoError(t, err)
	require.NotNil(t, database)
	defer cleanup()

	require.NoError(t, database.Ping(context.Background()))
}

func TestNewDB_WithTrace(t *testing.T) {
	driverName := registerFakeDriver(t)

	database, cleanup, err := NewDB(context.Background(),
		WithDriver(driverName),
		WithSource("fake"),
		WithTrace(),
	)
	require.NoError(t, err)
	require.NotNil(t, database)
	defer cleanup()

	require.NoError(t, database.Ping(context.Background()))
}
