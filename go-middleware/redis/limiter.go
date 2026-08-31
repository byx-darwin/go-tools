package redis

import (
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const defaultWaitPollInterval = 50 * time.Millisecond

// Limiter 基于 Redis 的分布式令牌桶限流器。
// 方法命名对齐 golang.org/x/time/rate.Limiter（Allow/AllowN/Wait），
// 但因跨网络调用 Redis，签名带 ctx 且返回 error。
type Limiter struct {
	client           goredis.UniversalClient
	key              string
	rate             float64
	burst            int
	waitPollInterval time.Duration
}

// LimiterOption 定义 Limiter 创建选项。
type LimiterOption func(*limiterOptions)

type limiterOptions struct {
	waitPollInterval time.Duration
}

// WithWaitPollInterval 设置 Wait 轮询重试间隔（必须 > 0，否则忽略）。
func WithWaitPollInterval(interval time.Duration) LimiterOption {
	return func(o *limiterOptions) {
		if interval > 0 {
			o.waitPollInterval = interval
		}
	}
}

// NewLimiter 创建分布式限流器。r 为每秒生成令牌数，burst 为桶容量。
// 默认配置：waitPollInterval=50ms。
func NewLimiter(client goredis.UniversalClient, key string, r float64, burst int, opts ...LimiterOption) *Limiter {
	o := &limiterOptions{waitPollInterval: defaultWaitPollInterval}
	for _, opt := range opts {
		opt(o)
	}
	return &Limiter{
		client:           client,
		key:              key,
		rate:             r,
		burst:            burst,
		waitPollInterval: o.waitPollInterval,
	}
}
