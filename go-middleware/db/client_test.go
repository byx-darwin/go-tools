package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"testing"
	"time"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
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

// pingFailConn 是一个实现 driver.Pinger 的 Conn，Ping 总是失败，用于测试
// NewDB 在 PingContext 失败时的清理与错误包装路径。
type pingFailConn struct{ fakeConn }

var errPingFailed = errors.New("ping failed")

func (pingFailConn) Ping(_ context.Context) error { return errPingFailed }

type pingFailDriver struct{}

func (pingFailDriver) Open(_ string) (driver.Conn, error) { return pingFailConn{}, nil }

var registerPingFailDriverOnce sync.Once

func registerPingFailDriver(t *testing.T) string {
	t.Helper()
	const name = "db_fake_driver_pingfail"
	registerPingFailDriverOnce.Do(func() {
		sql.Register(name, pingFailDriver{})
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

func TestWithPoolConfig(t *testing.T) {
	c := &dbConfig{}
	cfg := &Config{MaxOpenCons: 10, MaxIdleCons: 5}
	WithPoolConfig(cfg)(c)
	assert.Same(t, cfg, c.pool)
}

func TestWithPoolConfig_NilIgnored(t *testing.T) {
	c := &dbConfig{pool: &Config{MaxOpenCons: 1}}
	WithPoolConfig(nil)(c)
	assert.NotNil(t, c.pool)
	assert.Equal(t, 1, c.pool.MaxOpenCons)
}

func TestNewDB_WithPoolConfig_AppliesLimits(t *testing.T) {
	driverName := registerFakeDriver(t)

	database, cleanup, err := NewDB(context.Background(),
		WithDriver(driverName),
		WithSource("fake"),
		WithPoolConfig(&Config{
			MaxOpenCons:    3,
			MaxIdleCons:    2,
			ConMaxLifetime: time.Minute,
			MaxIdleTime:    30 * time.Second,
		}),
	)
	require.NoError(t, err)
	require.NotNil(t, database)
	defer cleanup()

	assert.Equal(t, 3, database.Stats().MaxOpenConnections)
}

func TestNewDBLegacy(t *testing.T) {
	driverName := registerFakeDriver(t)

	database, cleanup, err := NewDBLegacy(context.Background(), driverName, "fake",
		&Config{MaxOpenCons: 7})
	require.NoError(t, err)
	require.NotNil(t, database)
	defer cleanup()

	require.NoError(t, database.Ping(context.Background()))
	assert.Equal(t, 7, database.Stats().MaxOpenConnections)
}

func TestNewDB_OpenError_UnregisteredDriver(t *testing.T) {
	database, cleanup, err := NewDB(context.Background(),
		WithDriver("db_unregistered_driver_xyz"),
		WithSource("fake"),
	)
	require.Error(t, err)
	assert.Nil(t, database)
	assert.Nil(t, cleanup)
	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeOpen, code)
}

func TestNewDB_PingError_ClosesAndReturnsWrappedError(t *testing.T) {
	driverName := registerPingFailDriver(t)

	database, cleanup, err := NewDB(context.Background(),
		WithDriver(driverName),
		WithSource("fake"),
	)
	require.Error(t, err)
	assert.Nil(t, database)
	assert.Nil(t, cleanup)
	assert.ErrorIs(t, err, errPingFailed)
	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeConnect, code)
}
