// Package auth 提供认证存储的内存实现，用于开发和测试环境。
//
// MemorySessionStore 基于 samber/hot 缓存实现 session.Store 接口，
// MemoryDeviceStore 基于 sync.RWMutex + samber/hot 缓存实现 device.Store 接口。
// 两者均支持 TTL 自动过期和 Functional Options 配置。
package auth

import "time"

const (
	defaultSessionTTL = 30 * time.Minute
	defaultDeviceTTL  = 30 * 24 * time.Hour
	defaultMaxDevices = 5
	defaultCacheSize  = 1024
)

// config 存储配置（内部使用）。
type config struct {
	sessionTTL time.Duration
	deviceTTL  time.Duration
	maxDevices int
	cacheSize  int
}

// Option 定义配置选项函数。
type Option func(*config)

// WithSessionTTL 设置 Session 缓存的默认 TTL。
func WithSessionTTL(ttl time.Duration) Option {
	return func(c *config) {
		if ttl > 0 {
			c.sessionTTL = ttl
		}
	}
}

// WithDeviceTTL 设置设备会话缓存的默认 TTL。
func WithDeviceTTL(ttl time.Duration) Option {
	return func(c *config) {
		if ttl > 0 {
			c.deviceTTL = ttl
		}
	}
}

// WithMaxDevices 设置每个用户允许的最大设备数。AddDevice 超过限制时会踢出最旧设备。
func WithMaxDevices(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.maxDevices = n
		}
	}
}

// WithCacheSize 设置缓存容量（最大条目数）。
func WithCacheSize(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.cacheSize = n
		}
	}
}

// applyDefaults 创建默认配置并应用选项。
func applyDefaults(opts []Option) config {
	cfg := config{
		sessionTTL: defaultSessionTTL,
		deviceTTL:  defaultDeviceTTL,
		maxDevices: defaultMaxDevices,
		cacheSize:  defaultCacheSize,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}
