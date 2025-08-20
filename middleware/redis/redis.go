package redis

import (
	"context"
	"time"

	redisConfig "gitee.com/byx_darwin/go-tools/config/redis"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

func NewRedisClient(ctx context.Context,
	config *redisConfig.Config,
	isTrace bool) (*redis.Client, error) {
	options := new(redis.Options)
	options.Addr = config.Address[0]
	options.Password = config.Password
	options.DB = config.DB
	options.ContextTimeoutEnabled = true
	if config.PoolSize > 0 {
		options.PoolSize = config.PoolSize
	}
	if config.MinIdleCons > 0 {
		options.MinIdleConns = config.MinIdleCons
	}

	if config.MaxConnAge > 0 {
		options.MaxIdleConns = config.MaxConnAge
	}
	if config.IdleCheckFrequency > 0 {
		options.ConnMaxIdleTime = time.Duration(config.IdleCheckFrequency) * time.Millisecond
	}

	if config.IdleTimeout > 0 {
		options.ConnMaxLifetime = time.Duration(config.IdleTimeout) * time.Millisecond
	}

	if config.DialTimeout > 0 {
		options.DialTimeout = time.Duration(config.DialTimeout) * time.Millisecond
	}

	if config.ReadTimeout > 0 {
		options.ReadTimeout = time.Duration(config.ReadTimeout) * time.Millisecond
	}
	if config.WriteTimeout > 0 {
		options.WriteTimeout = time.Duration(config.WriteTimeout) * time.Millisecond
	}
	if config.PoolTimeout > 0 {
		options.PoolTimeout = time.Duration(config.PoolTimeout) * time.Millisecond
	}

	if config.MaxRetries > 0 {
		options.MaxRetries = config.MaxRetries
	}
	if config.MinRetryBackoff > 0 {
		options.MinRetryBackoff = time.Duration(config.MinRetryBackoff) * time.Millisecond
	}
	if config.MaxRetryBackoff > 0 {
		options.MaxRetryBackoff = time.Duration(config.MaxRetryBackoff) * time.Millisecond
	}
	client := redis.NewClient(options)
	if isTrace {
		// 开启 tracing instrumentation.
		if err := redisotel.InstrumentTracing(client); err != nil {
			panic(err)
		}
		// 开启 metrics instrumentation.
		if err := redisotel.InstrumentMetrics(client); err != nil {
			panic(err)
		}
	}
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err

	}
	return client, nil
}
