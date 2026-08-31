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
//
// 用法示例：
//
//	l := redis.NewLimiter(client, "limiter:api:user123", 10, 20) // 10 tokens/s, burst=20
//
//	// 非阻塞检查：
//	ok, err := l.Allow(ctx)
//	if err != nil {
//		return err
//	}
//	if !ok {
//		return errTooManyRequests
//	}
//
//	// 阻塞直到有令牌可用（或 ctx 取消）：
//	if err := l.Wait(ctx); err != nil {
//		return err
//	}
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
// r、burst 若为负数会被钳制为 0（rate=0 表示永不补充令牌，burst=0 表示每次请求都拒绝）。
// 默认配置：waitPollInterval=50ms。
func NewLimiter(client goredis.UniversalClient, key string, r float64, burst int, opts ...LimiterOption) *Limiter {
	o := &limiterOptions{waitPollInterval: defaultWaitPollInterval}
	for _, opt := range opts {
		opt(o)
	}
	if r < 0 {
		r = 0
	}
	if burst < 0 {
		burst = 0
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
// requested 理论上应恒为正（Go 层 AllowN 已拦截 n<=0），此处的 math.min(burst, tokens)
// 钳制是纵深防御：即便脚本被绕过 Go 层以负数 requested 调用，扣减后也不会残留超过 burst 的余量。
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
tokens = math.min(burst, tokens)

redis.call('hmset', tokens_key, 'tokens', tokens, 'last_refill_ts', now)
redis.call('pexpire', tokens_key, ttl)

return allowed
`)

// Allow 等价于 AllowN(ctx, 1)。
func (l *Limiter) Allow(ctx context.Context) (bool, error) {
	return l.AllowN(ctx, 1)
}

// AllowN 尝试消耗 n 个令牌，返回是否成功。
// n<=0 时直接返回 (true, nil) 且不访问 Redis，对齐 golang.org/x/time/rate.Limiter
// 对非正 n 的宽松处理；这也避免了负数 n 让令牌桶余量在 Lua 脚本中被误加而超过 burst 的问题。
func (l *Limiter) AllowN(ctx context.Context, n int) (bool, error) {
	if n <= 0 {
		return true, nil
	}
	now := time.Now().UnixMilli()
	res, err := tokenBucketScript.Run(ctx, l.client, []string{l.key}, l.rate, l.burst, now, n, l.ttlMillis()).Int64()
	if err != nil {
		return false, ErrLimiterEval.Wrap(err)
	}
	return res == 1, nil
}

// Wait 阻塞直到成功获取 1 个令牌，或 ctx 取消/超时返回其错误。
//
// ctx 取消时返回 ErrLimiterEval.Wrap(ctx.Err())（与 Mutex.Lock 保持一致的错误分类，
// 复用 code 20105，不新增错误码）：errors.Is(err, context.DeadlineExceeded) 或
// errors.Is(err, context.Canceled) 仍可通过该 wrap 链正常工作。
func (l *Limiter) Wait(ctx context.Context) error {
	for {
		ok, err := l.Allow(ctx)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}

		select {
		case <-ctx.Done():
			return ErrLimiterEval.Wrap(ctx.Err())
		case <-time.After(l.waitPollInterval):
		}
	}
}

// ttlMillis 计算 bucket key 的过期时间（约为完全补满所需时间的 2 倍，至少 1 秒），
// 避免闲置 key 常驻内存。rate <= 0 时无法计算有意义的补满时间（且理论上永不需要重新
// 补满），回退到 24 小时的下限：既避免除以零导致 +Inf/NaN 进而使 PEXPIRE 溢出或清空 key，
// 又避免像 1 秒下限那样在正常调用间隔内就把 key 的已耗尽状态错误地重置为满桶。
func (l *Limiter) ttlMillis() int64 {
	if l.rate <= 0 {
		return 24 * 60 * 60 * 1000
	}
	seconds := 2 * (float64(l.burst) / l.rate)
	if seconds < 1 {
		seconds = 1
	}
	return int64(seconds * 1000)
}
