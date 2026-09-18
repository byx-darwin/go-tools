package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

func TestClientOption_WithTrace(t *testing.T) {
	o := &clientOptions{}
	WithTrace()(o)
	if !o.trace {
		t.Error("WithTrace should set trace to true")
	}
}

func TestClientOption_DefaultNoTrace(t *testing.T) {
	o := &clientOptions{}
	// 不应用任何 option，默认不追踪
	if o.trace {
		t.Error("default trace should be false")
	}
}

func TestClientOption_MultipleOptions(t *testing.T) {
	o := &clientOptions{}
	opts := []ClientOption{WithTrace()}
	for _, opt := range opts {
		opt(o)
	}
	if !o.trace {
		t.Error("WithTrace applied via loop should set trace to true")
	}
}

func TestNewUniversalClient_SingleNode_Success(t *testing.T) {
	mr := miniredis.RunT(t)

	client, closeFn, err := NewUniversalClient(context.Background(), &Config{Addrs: []string{mr.Addr()}})

	require.NoError(t, err)
	require.NotNil(t, client)
	require.NotNil(t, closeFn)
	defer closeFn()

	assert.Equal(t, "PONG", must(client.Ping(context.Background()).Result()))
}

func TestNewUniversalClient_NilConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	// cfg == nil must fall back to an empty *Config rather than panicking.
	client, closeFn, err := NewUniversalClient(ctx, nil)

	if err != nil {
		assert.Nil(t, client)
		assert.Nil(t, closeFn)
		code, _ := goerror.Extract(err)
		assert.Equal(t, CodeConnect, code)
		return
	}
	require.NotNil(t, closeFn)
	closeFn()
}

func TestNewUniversalClient_Sentinel_PingError(t *testing.T) {
	mr := miniredis.RunT(t)
	// miniredis speaks plain Redis, not the Sentinel protocol, so a
	// FailoverClient talking to it must fail to Ping.
	cfg := &Config{
		MasterName:       "mymaster",
		SentinelUsername: "su",
		Addrs:            []string{mr.Addr()},
	}

	client, closeFn, err := NewUniversalClient(context.Background(), cfg)

	require.Error(t, err)
	assert.Nil(t, client)
	assert.Nil(t, closeFn)
	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeConnect, code)
}

func TestNewClient_Success(t *testing.T) {
	mr := miniredis.RunT(t)

	client, err := NewClient(context.Background(), &Config{Addrs: []string{mr.Addr()}})

	require.NoError(t, err)
	require.NotNil(t, client)
	_ = client.Close()
}

func TestNewClient_WithTrace_Success(t *testing.T) {
	mr := miniredis.RunT(t)

	client, err := NewClient(context.Background(), &Config{Addrs: []string{mr.Addr()}}, WithTrace())

	require.NoError(t, err)
	require.NotNil(t, client)
	_ = client.Close()
}

func TestNewClient_PingError(t *testing.T) {
	mr := miniredis.RunT(t)
	addr := mr.Addr()
	mr.Close() // nothing listening at addr anymore

	client, err := NewClient(context.Background(), &Config{Addrs: []string{addr}, DialTimeout: 100 * time.Millisecond})

	require.Error(t, err)
	assert.Nil(t, client)
	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeConnect, code)
}

func TestToFailoverOptions_AllFieldsSet(t *testing.T) {
	cfg := &Config{
		MasterName:       "mymaster",
		SentinelUsername: "su",
		SentinelPassword: "sp",
		Addrs:            []string{"s1:26379", "s2:26379"},
		Password:         "redis-pass",
		DB:               2,
		Username:         "redis-user",
		ClientName:       "myapp",
		PoolSize:         50,
		MinIdleConns:     5,
		DialTimeout:      1 * time.Second,
		ReadTimeout:      2 * time.Second,
		WriteTimeout:     3 * time.Second,
		PoolTimeout:      4 * time.Second,
		ConnMaxIdleTime:  5 * time.Minute,
		ConnMaxLifetime:  30 * time.Minute,
		MaxRetries:       3,
		MinRetryBackoff:  8 * time.Millisecond,
		MaxRetryBackoff:  512 * time.Millisecond,
	}
	cfg.TLS.Enable = true
	cfg.TLS.InsecureSkipVerify = true

	opts := toFailoverOptions(cfg)

	assert.Equal(t, []string{"s1:26379", "s2:26379"}, opts.SentinelAddrs)
	assert.Equal(t, "redis-user", opts.Username)
	assert.Equal(t, 2, opts.DB)
	assert.Equal(t, "myapp", opts.ClientName)
	assert.Equal(t, 50, opts.PoolSize)
	assert.Equal(t, 5, opts.MinIdleConns)
	assert.Equal(t, 1*time.Second, opts.DialTimeout)
	assert.Equal(t, 2*time.Second, opts.ReadTimeout)
	assert.Equal(t, 3*time.Second, opts.WriteTimeout)
	assert.Equal(t, 4*time.Second, opts.PoolTimeout)
	assert.Equal(t, 5*time.Minute, opts.ConnMaxIdleTime)
	assert.Equal(t, 30*time.Minute, opts.ConnMaxLifetime)
	assert.Equal(t, 3, opts.MaxRetries)
	assert.Equal(t, 8*time.Millisecond, opts.MinRetryBackoff)
	assert.Equal(t, 512*time.Millisecond, opts.MaxRetryBackoff)
	require.NotNil(t, opts.TLSConfig)
	assert.True(t, opts.TLSConfig.InsecureSkipVerify)
}

func TestApplyOpts_ProtocolIdleCheckAndTLS(t *testing.T) {
	opts := &redis.Options{}
	cfg := &Config{
		Protocol:           3,
		IdleCheckFrequency: 90 * time.Second,
	}
	cfg.TLS.Enable = true
	cfg.TLS.InsecureSkipVerify = false

	applyOpts(opts, cfg)

	assert.Equal(t, 3, opts.Protocol)
	assert.Equal(t, 90*time.Second, opts.ConnMaxIdleTime)
	require.NotNil(t, opts.TLSConfig)
	assert.False(t, opts.TLSConfig.InsecureSkipVerify)
}

func TestApplyOpts_RemainingTimeoutFields(t *testing.T) {
	opts := &redis.Options{}
	cfg := &Config{
		WriteTimeout:    3 * time.Second,
		PoolTimeout:     4 * time.Second,
		ConnMaxIdleTime: 5 * time.Minute,
		ConnMaxLifetime: 30 * time.Minute,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
	}

	applyOpts(opts, cfg)

	assert.Equal(t, 3*time.Second, opts.WriteTimeout)
	assert.Equal(t, 4*time.Second, opts.PoolTimeout)
	assert.Equal(t, 5*time.Minute, opts.ConnMaxIdleTime)
	assert.Equal(t, 30*time.Minute, opts.ConnMaxLifetime)
	assert.Equal(t, 8*time.Millisecond, opts.MinRetryBackoff)
	assert.Equal(t, 512*time.Millisecond, opts.MaxRetryBackoff)
}

func must(s string, err error) string {
	if err != nil {
		panic(err)
	}
	return s
}
