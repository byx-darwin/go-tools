package captcha

import (
	"sync"
	"time"

	"github.com/byx-darwin/go-tools/go-common/cache"
)

const (
	defaultPreKey     = "CAPTCHA_"
	defaultExpiration = 5 * time.Minute
	defaultCapacity   = 1024
)

// Option 定义配置选项函数。
type Option func(*CacheStore)

// WithExpiration 设置过期时间。
func WithExpiration(expiration time.Duration) Option {
	return func(c *CacheStore) {
		if expiration > 0 {
			c.expiration = expiration
		}
	}
}

// WithPreKey 设置 key 前缀。
func WithPreKey(preKey string) Option {
	return func(c *CacheStore) {
		if preKey != "" {
			c.preKey = preKey
		}
	}
}

// WithCapacity 设置缓存容量。
func WithCapacity(capacity int) Option {
	return func(c *CacheStore) {
		if capacity > 0 {
			c.capacity = capacity
		}
	}
}

// WithEvictionPolicy 设置淘汰策略（FIFO / LRU / LFU / CLOCK / MRU）。
func WithEvictionPolicy(policy cache.EvictionAlgorithm) Option {
	return func(c *CacheStore) {
		c.evictionPolicy = policy
	}
}

// CacheStore 验证码缓存存储，实现 base64Captcha.Store 接口。
// 底层 HotCache 自带并发安全（sync.RWMutex），此处仅对运行时可变的
// expiration / preKey 加锁保护。
type CacheStore struct {
	expiration     time.Duration
	preKey         string
	capacity       int
	evictionPolicy cache.EvictionAlgorithm
	cache          *cache.HotCache[string, string]
	mu             sync.RWMutex
}

// NewCacheStore 创建验证码缓存存储，默认配置：
//   - 过期时间: 5 分钟
//   - 前缀: "CAPTCHA_"
//   - 容量: 1024
//   - 淘汰策略: FIFO
func NewCacheStore(opts ...Option) *CacheStore {
	store := &CacheStore{
		expiration:     defaultExpiration,
		preKey:         defaultPreKey,
		capacity:       defaultCapacity,
		evictionPolicy: cache.FIFO,
	}
	for _, opt := range opts {
		opt(store)
	}
	store.cache = cache.New[string, string](store.evictionPolicy, store.capacity).Build()
	return store
}

// NewCacheStoreWithConfig 保留向后兼容。
//
// Deprecated: 使用 NewCacheStore 配合 Options 替代。
func NewCacheStoreWithConfig(length int, expiration time.Duration, preKey string) *CacheStore {
	return NewCacheStore(
		WithCapacity(length),
		WithExpiration(expiration),
		WithPreKey(preKey),
	)
}

// NewCacheStoreWithTTL 保留向后兼容。
//
// Deprecated: 使用 NewCacheStore 配合 Options 替代。
func NewCacheStoreWithTTL(length int, expiration time.Duration) *CacheStore {
	return NewCacheStore(
		WithCapacity(length),
		WithExpiration(expiration),
	)
}

// Expiration 返回当前过期时间（并发安全）。
func (c *CacheStore) Expiration() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.expiration
}

// PreKey 返回当前 key 前缀（并发安全）。
func (c *CacheStore) PreKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.preKey
}

// SetPreKey 更新 key 前缀（并发安全）。
func (c *CacheStore) SetPreKey(preKey string) {
	if preKey == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.preKey = preKey
}

// SetExpiration 更新过期时间（并发安全）。
func (c *CacheStore) SetExpiration(expiration time.Duration) {
	if expiration <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.expiration = expiration
}

// Update 批量更新配置（并发安全）。
func (c *CacheStore) Update(opts ...Option) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, opt := range opts {
		opt(c)
	}
}

// snapshot 内部快照当前 preKey 和 expiration，避免多次加锁。
func (c *CacheStore) snapshot() (string, time.Duration) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.preKey, c.expiration
}

// Set 实现 base64Captcha.Store 接口。
func (c *CacheStore) Set(id string, value string) error {
	preKey, expiration := c.snapshot()
	c.cache.SetWithTTL(preKey+id, value, expiration)
	return nil
}

// Get 实现 base64Captcha.Store 接口。
func (c *CacheStore) Get(id string, clear bool) string {
	preKey, _ := c.snapshot()
	key := preKey + id
	val, ok, _ := c.cache.Get(key)
	if !ok {
		return ""
	}
	if clear {
		c.cache.Delete(key)
	}
	return val
}

// Verify 实现 base64Captcha.Store 接口。
func (c *CacheStore) Verify(id, answer string, clear bool) bool {
	v := c.Get(id, clear)
	return v == answer
}

// GetAndDelete 原子性获取并删除。
func (c *CacheStore) GetAndDelete(id string) (string, bool) {
	preKey, _ := c.snapshot()
	key := preKey + id
	val, ok, _ := c.cache.Get(key)
	if !ok {
		return "", false
	}
	c.cache.Delete(key)
	return val, true
}

// Clear 清空缓存。
func (c *CacheStore) Clear() {
	c.cache.Purge()
}

// Len 返回当前缓存条目数。
func (c *CacheStore) Len() int {
	return c.cache.Len()
}
