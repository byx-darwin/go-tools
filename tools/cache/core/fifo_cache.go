package core

import (
	"gitee.com/byx_darwin/go-tools/tools/cache"
	genericsCache "github.com/Code-Hex/go-generics-cache"
	"github.com/Code-Hex/go-generics-cache/policy/fifo"
	"time"
)

// FifoCache 是一个泛型缓存结构体，它封装了 genericsCache.Cache 接口，提供了先进先出（FIFO）策略的缓存功能。
// 该缓存适用于需要按照访问顺序管理键值对的场景。
type FifoCache[K comparable, V any] struct {
	cache *genericsCache.Cache[K, V] // 内部缓存实例
}

// Get 方法用于从缓存中获取指定键的值。如果键存在，则返回对应的值和一个表示找到的布尔值。
func (c FifoCache[K, V]) Get(key K) (value V, ok bool) {
	return c.cache.Get(key)
}

// Set 方法用于向缓存中设置键值对。
func (c FifoCache[K, V]) Set(key K, val V) {
	c.cache.Set(key, val)
}

// SetWithExp 方法是 Set 方法的扩展，它允许为设置的键值对指定过期时间。
func (c FifoCache[K, V]) SetWithExp(key K, val V, exp time.Duration) {
	c.cache.Set(key, val, genericsCache.WithExpiration(exp))
}

// Delete 方法用于从缓存中删除指定的键及其对应的值。
func (c FifoCache[K, V]) Delete(key K) {
	c.cache.Delete(key)
}

// Keys 方法返回缓存中所有键的切片，用于遍历缓存中的所有键。
func (c FifoCache[K, V]) Keys() []K {
	return c.cache.Keys()
}

// Contains 方法检查缓存中是否存在指定的键。
func (c FifoCache[K, V]) Contains(key K) bool {
	return c.cache.Contains(key)
}

// Empty 方法用于清空缓存，即删除缓存中的所有键值对。
func (c FifoCache[K, V]) Empty() {
	keys := c.Keys()
	for _, key := range keys {
		c.Delete(key)
	}
}

// Len 方法返回缓存中键值对的数量。
func (c FifoCache[K, V]) Len() int {
	return len(c.cache.Keys())
}

// NewFifoCache 是 FifoCache 的构造函数，它接受一个容量参数，并返回一个初始化了的 FifoCache 实例。
func NewFifoCache[K comparable, V any](capacity int) cache.Cache[K, V] {
	opts := genericsCache.AsFIFO[K, V](fifo.WithCapacity(capacity)) // 创建带有容量限制的 FIFO 缓存选项
	c := genericsCache.New[K, V](opts)                              // 根据选项创建缓存实例
	return &FifoCache[K, V]{ // 返回 FifoCache 包装后的缓存实例
		c,
	}
}
