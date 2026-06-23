package redis

import (
	"context"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

// Client Redis 客户端接口（嵌入 UniversalClient，支持单节点/哨兵/集群）。
type Client interface {
	redis.UniversalClient
}

// NewUniversalClient 创建 Redis UniversalClient。
// 根据配置自动选择单节点、哨兵或集群模式。
// 返回 (client, closeFunc, error)。
func NewUniversalClient(ctx context.Context, cfg *Config) (Client, func(), error) {
	if cfg == nil {
		cfg = &Config{}
	}

	var rdb redis.UniversalClient

	if cfg.MasterName != "" {
		// Sentinel 模式
		rdb = redis.NewFailoverClient(toFailoverOptions(cfg))
	} else {
		// 单节点模式
		rdb = redis.NewClient(toOptions(cfg))
	}

	closeFn := func() {
		_ = rdb.Close()
	}

	if _, err := rdb.Ping(ctx).Result(); err != nil {
		closeFn()
		return nil, nil, err
	}

	return rdb, closeFn, nil
}

// NewClient 创建带 OpenTelemetry 追踪的 Redis Client。
// 如果 isTrace 为 true，注册 redisotel 追踪和指标。
func NewClient(ctx context.Context, cfg *Config, isTrace bool) (*redis.Client, error) {
	client := redis.NewClient(toOptions(cfg))
	if isTrace {
		if err := redisotel.InstrumentTracing(client); err != nil {
			panic(err)
		}
		if err := redisotel.InstrumentMetrics(client); err != nil {
			panic(err)
		}
	}
	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, err
	}
	return client, nil
}

func toOptions(cfg *Config) *redis.Options {
	opts := &redis.Options{
		ContextTimeoutEnabled: true,
	}
	if len(cfg.Addrs) > 0 {
		opts.Addr = cfg.Addrs[0]
	}
	applyOpts(opts, cfg)
	return opts
}

func toFailoverOptions(cfg *Config) *redis.FailoverOptions {
	opts := &redis.FailoverOptions{
		ContextTimeoutEnabled: true,
		MasterName:            cfg.MasterName,
	}
	if cfg.SentinelUsername != "" {
		opts.SentinelUsername = cfg.SentinelUsername
	}
	if cfg.SentinelPassword != "" {
		opts.SentinelPassword = cfg.SentinelPassword
	}
	if len(cfg.Addrs) > 0 {
		opts.SentinelAddrs = cfg.Addrs
	}
	opts.Password = cfg.Password
	opts.DB = cfg.DB
	opts.Username = cfg.Username
	if cfg.ClientName != "" {
		opts.ClientName = cfg.ClientName
	}
	if cfg.PoolSize > 0 {
		opts.PoolSize = cfg.PoolSize
	}
	if cfg.MinIdleConns > 0 {
		opts.MinIdleConns = cfg.MinIdleConns
	}
	if cfg.DialTimeout > 0 {
		opts.DialTimeout = cfg.DialTimeout
	}
	if cfg.ReadTimeout > 0 {
		opts.ReadTimeout = cfg.ReadTimeout
	}
	if cfg.WriteTimeout > 0 {
		opts.WriteTimeout = cfg.WriteTimeout
	}
	if cfg.PoolTimeout > 0 {
		opts.PoolTimeout = cfg.PoolTimeout
	}
	if cfg.ConnMaxIdleTime > 0 {
		opts.ConnMaxIdleTime = cfg.ConnMaxIdleTime
	}
	if cfg.ConnMaxLifetime > 0 {
		opts.ConnMaxLifetime = cfg.ConnMaxLifetime
	}
	if cfg.MaxRetries > 0 {
		opts.MaxRetries = cfg.MaxRetries
	}
	if cfg.MinRetryBackoff > 0 {
		opts.MinRetryBackoff = cfg.MinRetryBackoff
	}
	if cfg.MaxRetryBackoff > 0 {
		opts.MaxRetryBackoff = cfg.MaxRetryBackoff
	}
	return opts
}

func applyOpts(opts *redis.Options, cfg *Config) {
	opts.Password = cfg.Password
	opts.DB = cfg.DB
	opts.Username = cfg.Username
	if cfg.ClientName != "" {
		opts.ClientName = cfg.ClientName
	}
	if cfg.Protocol > 0 {
		opts.Protocol = cfg.Protocol
	}
	if cfg.PoolSize > 0 {
		opts.PoolSize = cfg.PoolSize
	}
	if cfg.MinIdleConns > 0 {
		opts.MinIdleConns = cfg.MinIdleConns
	}
	if cfg.DialTimeout > 0 {
		opts.DialTimeout = cfg.DialTimeout
	}
	if cfg.ReadTimeout > 0 {
		opts.ReadTimeout = cfg.ReadTimeout
	}
	if cfg.WriteTimeout > 0 {
		opts.WriteTimeout = cfg.WriteTimeout
	}
	if cfg.PoolTimeout > 0 {
		opts.PoolTimeout = cfg.PoolTimeout
	}
	if cfg.ConnMaxIdleTime > 0 {
		opts.ConnMaxIdleTime = cfg.ConnMaxIdleTime
	}
	if cfg.ConnMaxLifetime > 0 {
		opts.ConnMaxLifetime = cfg.ConnMaxLifetime
	}
	if cfg.IdleCheckFrequency > 0 {
		opts.ConnMaxIdleTime = cfg.IdleCheckFrequency // fallback: same field
	}
	if cfg.MaxRetries > 0 {
		opts.MaxRetries = cfg.MaxRetries
	}
	if cfg.MinRetryBackoff > 0 {
		opts.MinRetryBackoff = cfg.MinRetryBackoff
	}
	if cfg.MaxRetryBackoff > 0 {
		opts.MaxRetryBackoff = cfg.MaxRetryBackoff
	}
}
