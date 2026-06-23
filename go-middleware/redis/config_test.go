package redis

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestConfig_Defaults(t *testing.T) {
	c := &Config{}
	assert.Empty(t, c.Addrs)
	assert.Empty(t, c.MasterName)
	assert.Equal(t, 0, c.Protocol)
}

func TestConfig_Addrs(t *testing.T) {
	c := Config{
		Addrs: []string{"redis1:6379", "redis2:6379"},
	}
	assert.Equal(t, []string{"redis1:6379", "redis2:6379"}, c.Addrs)
}

func TestConfig_Sentinel(t *testing.T) {
	c := Config{
		MasterName:       "mymaster",
		SentinelUsername: "sentinel-user",
		SentinelPassword: "sentinel-pass",
		Addrs:            []string{"sentinel1:26379", "sentinel2:26379"},
	}
	assert.Equal(t, "mymaster", c.MasterName)
	assert.Equal(t, "sentinel-user", c.SentinelUsername)
	assert.Equal(t, "sentinel-pass", c.SentinelPassword)
}

func TestConfig_Durations(t *testing.T) {
	c := Config{
		DialTimeout:       5 * time.Second,
		ReadTimeout:       3 * time.Second,
		WriteTimeout:      3 * time.Second,
		ConnMaxIdleTime:   5 * time.Minute,
		ConnMaxLifetime:   30 * time.Minute,
		MinRetryBackoff:   8 * time.Millisecond,
		MaxRetryBackoff:   512 * time.Millisecond,
	}

	assert.Equal(t, 5*time.Second, c.DialTimeout)
	assert.Equal(t, 3*time.Second, c.ReadTimeout)
	assert.Equal(t, 5*time.Minute, c.ConnMaxIdleTime)
	assert.Equal(t, 30*time.Minute, c.ConnMaxLifetime)
}

func TestToOptions(t *testing.T) {
	cfg := &Config{
		Addrs:    []string{"localhost:6379"},
		Password: "secret",
		DB:       3,
		PoolSize: 50,
	}

	opts := toOptions(cfg)
	assert.Equal(t, "localhost:6379", opts.Addr)
	assert.Equal(t, "secret", opts.Password)
	assert.Equal(t, 3, opts.DB)
	assert.Equal(t, 50, opts.PoolSize)
}

func TestToFailoverOptions(t *testing.T) {
	cfg := &Config{
		MasterName:       "mymaster",
		SentinelUsername: "su",
		SentinelPassword: "sp",
		Addrs:            []string{"s1:26379"},
		Password:         "redis-pass",
	}

	opts := toFailoverOptions(cfg)
	assert.Equal(t, "mymaster", opts.MasterName)
	assert.Equal(t, "su", opts.SentinelUsername)
	assert.Equal(t, "sp", opts.SentinelPassword)
	assert.Equal(t, "redis-pass", opts.Password)
}

func TestApplyOpts_ZeroValuesIgnore(t *testing.T) {
	opts := &redis.Options{PoolSize: 100} // default
	applyOpts(opts, &Config{PoolSize: 0})  // zero → keep default
	assert.Equal(t, 100, opts.PoolSize)
}

func TestApplyOpts_Overrides(t *testing.T) {
	opts := &redis.Options{}
	cfg := &Config{
		ClientName:     "myapp",
		Protocol:       3,
		MaxRetries:     5,
		MinIdleConns:   10,
		DialTimeout:    5 * time.Second,
		ReadTimeout:    3 * time.Second,
	}

	applyOpts(opts, cfg)
	assert.Equal(t, "myapp", opts.ClientName)
	assert.Equal(t, 3, opts.Protocol)
	assert.Equal(t, 5, opts.MaxRetries)
	assert.Equal(t, 10, opts.MinIdleConns)
	assert.Equal(t, 5*time.Second, opts.DialTimeout)
}
