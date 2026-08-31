package redis

import (
	"context"
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

// tokenBucketScript 原子化令牌桶脚本：按 elapsed 时间补充令牌（上限 burst），
// 若余量 >= requested 则扣减并放行，否则拒绝；同时刷新 key 的过期时间。
var tokenBucketScript = goredis.NewScript(`
local tokens_key = KEYS[1]
local rate = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])
local ttl = tonumber(ARGV[5])

local data = redis.call('hmget', tokens_key, 'tokens', 'last_refill_ts')
local tokens = tonumber(data[1])
local last_refill_ts = tonumber(data[2])

if tokens == nil then
	tokens = burst
	last_refill_ts = now
end

local elapsed = math.max(0, now - last_refill_ts)
local refill = (elapsed / 1000) * rate
tokens = math.min(burst, tokens + refill)

local allowed = 0
if tokens >= requested then
	tokens = tokens - requested
	allowed = 1
end

redis.call('hmset', tokens_key, 'tokens', tokens, 'last_refill_ts', now)
redis.call('pexpire', tokens_key, ttl)

return allowed
`)

// Allow 等价于 AllowN(ctx, 1)。
func (l *Limiter) Allow(ctx context.Context) (bool, error) {
	return l.AllowN(ctx, 1)
}

// AllowN 尝试消耗 n 个令牌，返回是否成功。
func (l *Limiter) AllowN(ctx context.Context, n int) (bool, error) {
	now := time.Now().UnixMilli()
	res, err := tokenBucketScript.Run(ctx, l.client, []string{l.key}, l.rate, l.burst, now, n, l.ttlMillis()).Int64()
	if err != nil {
		return false, ErrLimiterEval.Wrap(err)
	}
	return res == 1, nil
}

// ttlMillis 计算 bucket key 的过期时间（约为完全补满所需时间的 2 倍，至少 1 秒），
// 避免闲置 key 常驻内存。
func (l *Limiter) ttlMillis() int64 {
	seconds := 2 * (float64(l.burst) / l.rate)
	if seconds < 1 {
		seconds = 1
	}
	return int64(seconds * 1000)
}
