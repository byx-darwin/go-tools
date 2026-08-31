package redis

import (
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	defaultMutexTTL      = 10 * time.Second
	defaultRetryInterval = 100 * time.Millisecond
)

// Mutex 基于 Redis 的分布式互斥锁（单实例 SET NX PX 方案），不支持可重入。
// 一个 Mutex 实例代表一次加锁会话：Lock/TryLock 成功后必须调用 Unlock 才能再次加锁。
type Mutex struct {
	client        goredis.UniversalClient
	key           string
	ttl           time.Duration
	retryInterval time.Duration
	watchdog      bool

	mu     sync.Mutex
	token  string
	stopCh chan struct{}
}

// MutexOption 定义 Mutex 创建选项。
type MutexOption func(*mutexOptions)

type mutexOptions struct {
	ttl           time.Duration
	retryInterval time.Duration
	watchdog      bool
}

// WithTTL 设置锁的过期时间（必须 > 0，否则忽略，保留默认值）。
func WithTTL(ttl time.Duration) MutexOption {
	return func(o *mutexOptions) {
		if ttl > 0 {
			o.ttl = ttl
		}
	}
}

// WithRetryInterval 设置 Lock 阻塞重试的轮询间隔（必须 > 0，否则忽略）。
func WithRetryInterval(interval time.Duration) MutexOption {
	return func(o *mutexOptions) {
		if interval > 0 {
			o.retryInterval = interval
		}
	}
}

// WithWatchdog 设置是否启用续期 watchdog（默认 true）。
func WithWatchdog(enabled bool) MutexOption {
	return func(o *mutexOptions) {
		o.watchdog = enabled
	}
}

// NewMutex 创建分布式锁实例。
// 默认配置：ttl=10s，retryInterval=100ms，watchdog=true。
func NewMutex(client goredis.UniversalClient, key string, opts ...MutexOption) *Mutex {
	o := &mutexOptions{
		ttl:           defaultMutexTTL,
		retryInterval: defaultRetryInterval,
		watchdog:      true,
	}
	for _, opt := range opts {
		opt(o)
	}
	return &Mutex{
		client:        client,
		key:           key,
		ttl:           o.ttl,
		retryInterval: o.retryInterval,
		watchdog:      o.watchdog,
	}
}
