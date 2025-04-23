package core

import (
	"gitcode.com/sznc/go-tools/tools/cache"
	genericsCache "github.com/Code-Hex/go-generics-cache"
	"github.com/Code-Hex/go-generics-cache/policy/lfu"
	"time"
)

type LfuCache[K comparable, V any] struct {
	cache *genericsCache.Cache[K, V]
}

func (c LfuCache[K, V]) Get(key K) (value V, ok bool) {
	return c.cache.Get(key)
}

func (c LfuCache[K, V]) Set(key K, val V) {
	c.cache.Set(key, val)
}

func (c LfuCache[K, V]) SetWithExp(key K, val V, exp time.Duration) {
	c.cache.Set(key, val, genericsCache.WithExpiration(exp))
}

func (c LfuCache[K, V]) Delete(key K) {
	c.cache.Delete(key)
}

func (c LfuCache[K, V]) Keys() []K {
	return c.cache.Keys()
}
func (c LfuCache[K, V]) Contains(key K) bool {
	return c.cache.Contains(key)
}
func (c LfuCache[K, V]) Empty() {
	keys := c.Keys()
	for _, key := range keys {
		c.Delete(key)
	}
}
func (c LfuCache[K, V]) Len() int {
	return len(c.cache.Keys())
}
func NewLfuCache[K comparable, V any](capacity int) cache.Cache[K, V] {
	opts := genericsCache.AsLFU[K, V](lfu.WithCapacity(capacity))
	c := genericsCache.New[K, V](opts)
	return &LfuCache[K, V]{
		c,
	}
}
