package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
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

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// TryLock 单次非阻塞尝试获取锁。成功后若启用 watchdog 会自动启动续期。
func (m *Mutex) TryLock(ctx context.Context) (bool, error) {
	token, err := randomToken()
	if err != nil {
		return false, ErrLockAcquire.Wrap(err)
	}

	ok, err := m.client.SetNX(ctx, m.key, token, m.ttl).Result()
	if err != nil {
		return false, ErrLockAcquire.Wrap(err)
	}
	if !ok {
		return false, nil
	}

	m.mu.Lock()
	m.token = token
	m.mu.Unlock()

	if m.watchdog {
		m.startWatchdog()
	}
	return true, nil
}

var renewScript = goredis.NewScript(`
if redis.call('get', KEYS[1]) == ARGV[1] then
	return redis.call('pexpire', KEYS[1], ARGV[2])
else
	return 0
end
`)

// startWatchdog 启动后台续期 goroutine：按 ttl/3 周期执行 Lua 续期脚本，
// 直到 stopWatchdog 被调用或续期失败（锁已丢失）。
func (m *Mutex) startWatchdog() {
	stop := make(chan struct{})
	m.mu.Lock()
	m.stopCh = stop
	m.mu.Unlock()

	interval := m.ttl / 3
	if interval <= 0 {
		interval = time.Millisecond
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				m.mu.Lock()
				token := m.token
				m.mu.Unlock()
				if token == "" {
					return
				}

				res, err := renewScript.Run(context.Background(), m.client, []string{m.key}, token, m.ttl.Milliseconds()).Int64()
				if err != nil || res == 0 {
					return
				}
			}
		}
	}()
}

// releaseScript 校验 KEYS[1] 的值等于 ARGV[1]（持有者 token）后再删除，避免误删他人持有的锁。
var releaseScript = goredis.NewScript(`
if redis.call('get', KEYS[1]) == ARGV[1] then
	return redis.call('del', KEYS[1])
else
	return 0
end
`)

// Unlock 释放锁：停止 watchdog，Lua 脚本校验 token 匹配后删除 key。
// 若锁未持有、已过期，或被其他持有者持有，返回 ErrLockRelease。
func (m *Mutex) Unlock(ctx context.Context) error {
	m.stopWatchdog()

	m.mu.Lock()
	token := m.token
	m.mu.Unlock()

	if token == "" {
		return ErrLockRelease.Wrap(errors.New("lock not held"))
	}

	res, err := releaseScript.Run(ctx, m.client, []string{m.key}, token).Int64()
	if err != nil {
		return ErrLockRelease.Wrap(err)
	}
	if res == 0 {
		return ErrLockRelease.Wrap(errors.New("lock not held or already expired"))
	}

	m.mu.Lock()
	m.token = ""
	m.mu.Unlock()
	return nil
}

// stopWatchdog 停止续期 goroutine（幂等，可安全重复调用）。
func (m *Mutex) stopWatchdog() {
	m.mu.Lock()
	stop := m.stopCh
	m.stopCh = nil
	m.mu.Unlock()

	if stop != nil {
		close(stop)
	}
}

// Lock 阻塞获取锁，直到成功或 ctx 取消。成功后自动启动 watchdog（若启用）。
func (m *Mutex) Lock(ctx context.Context) error {
	ticker := time.NewTicker(m.retryInterval)
	defer ticker.Stop()

	for {
		ok, err := m.TryLock(ctx)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}

		select {
		case <-ctx.Done():
			return ErrLockAcquire.Wrap(ctx.Err())
		case <-ticker.C:
		}
	}
}
