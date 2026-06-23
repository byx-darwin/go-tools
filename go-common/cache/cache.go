package cache

import "github.com/samber/hot"

// EvictionAlgorithm 淘汰算法类型（别名）
type EvictionAlgorithm = hot.EvictionAlgorithm

const (
	LRU      = hot.LRU
	LFU      = hot.LFU
	FIFO     = hot.FIFO
	TwoQueue = hot.TwoQueue
	ARC      = hot.ARC
)

// HotCache 泛型缓存类型别名，直接暴露 samber/hot 的 HotCache。
//
// 支持 LRU/LFU/FIFO/TwoQueue/ARC 淘汰策略，可选 TTL / 分片 / 加载器。
//
//	// 简单用法
//	c := cache.New[string, int](cache.LRU, 100)
//	c.Set("k", 42)
//	v, ok, _ := c.Get("k")
//
//	// 带 TTL
//	c := cache.New[string, int](cache.LRU, 100).WithTTL(time.Minute).Build()
type HotCache[K comparable, V any] = hot.HotCache[K, V]

// HotCacheConfig 构建器类型别名
type HotCacheConfig[K comparable, V any] = hot.HotCacheConfig[K, V]

// New 创建缓存构建器（设置淘汰策略和容量）。
// 调用 .Build() 获得 *HotCache[K, V]。
func New[K comparable, V any](algorithm EvictionAlgorithm, capacity int) HotCacheConfig[K, V] {
	return hot.NewHotCache[K, V](algorithm, capacity)
}
