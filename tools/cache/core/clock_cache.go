package core

import (
	"gitcode.com/sznc/go-tools/tools/cache"
	genericsCache "github.com/Code-Hex/go-generics-cache"
	"github.com/Code-Hex/go-generics-cache/policy/clock"
	"time"
)

// ClockCache 是一个泛型缓存结构体，它封装了 genericsCache.Cache 接口，提供了带有时间戳的缓存功能。
// 该缓存可以设置过期时间，并在过期后自动删除，适用于需要缓存但又不希望数据无限期驻留的场景。
type ClockCache[K comparable, V any] struct {
	cache *genericsCache.Cache[K, V] // 内部缓存实例
}

// Get 方法用于从缓存中获取指定键的值。如果键存在且未过期，则返回对应的值和一个表示找到的布尔值。
func (c ClockCache[K, V]) Get(key K) (value V, ok bool) {
	return c.cache.Get(key)
}

// Set 方法用于向缓存中设置键值对。可以指定键的过期时间。
func (c ClockCache[K, V]) Set(key K, val V) {
	c.cache.Set(key, val)
}

// SetWithExp 方法是 Set 方法的扩展，它允许为设置的键值对指定过期时间。
func (c ClockCache[K, V]) SetWithExp(key K, val V, exp time.Duration) {
	c.cache.Set(key, val, genericsCache.WithExpiration(exp))
}

// Delete 方法用于从缓存中删除指定的键及其对应的值。
func (c ClockCache[K, V]) Delete(key K) {
	c.cache.Delete(key)
}

// Keys 方法返回缓存中所有键的切片，用于遍历缓存中的所有键。
func (c ClockCache[K, V]) Keys() []K {
	return c.cache.Keys()
}

// Contains 方法检查缓存中是否存在指定的键。
func (c ClockCache[K, V]) Contains(key K) bool {
	return c.cache.Contains(key)
}

// Empty 方法用于清空缓存，即删除缓存中的所有键值对。
func (c ClockCache[K, V]) Empty() {
	keys := c.Keys()
	for _, key := range keys {
		c.Delete(key)
	}
}

// Len 方法返回缓存中键值对的数量。
func (c ClockCache[K, V]) Len() int {
	return len(c.cache.Keys())
}

// NewClockCache 是 ClockCache 的构造函数，它接受一个容量参数，并返回一个初始化了的 ClockCache 实例。
func NewClockCache[K comparable, V any](capacity int) cache.Cache[K, V] {
	opts := genericsCache.AsClock[K, V](clock.WithCapacity(capacity)) // 创建带有容量限制的缓存选项
	c := genericsCache.New[K, V](opts)                                // 根据选项创建缓存实例
	return &ClockCache[K, V]{                                         // 返回 ClockCache 包装后的缓存实例
		c,
	}
}
