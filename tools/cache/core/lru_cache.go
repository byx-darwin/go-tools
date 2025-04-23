package core

import (
	"gitcode.com/sznc/go-tools/tools/cache"
	genericsCache "github.com/Code-Hex/go-generics-cache"
	"github.com/Code-Hex/go-generics-cache/policy/lru"
	"time"
)

type LruCache[K comparable, V any] struct {
	cache *genericsCache.Cache[K, V]
}

func (c LruCache[K, V]) Get(key K) (value V, ok bool) {
	return c.cache.Get(key)
}

func (c LruCache[K, V]) Set(key K, val V) {
	c.cache.Set(key, val)
}

func (c LruCache[K, V]) SetWithExp(key K, val V, exp time.Duration) {
	c.cache.Set(key, val, genericsCache.WithExpiration(exp))
}

func (c LruCache[K, V]) Delete(key K) {
	c.cache.Delete(key)
}

func (c LruCache[K, V]) Keys() []K {
	return c.cache.Keys()
}
func (c LruCache[K, V]) Contains(key K) bool {
	return c.cache.Contains(key)
}
func (c LruCache[K, V]) Empty() {
	keys := c.Keys()
	for _, key := range keys {
		c.Delete(key)
	}
}
func (c LruCache[K, V]) Len() int {
	return len(c.cache.Keys())
}
func NewLruCache[K comparable, V any](capacity int) cache.Cache[K, V] {
	opts := genericsCache.AsLRU[K, V](lru.WithCapacity(capacity))
	c := genericsCache.New[K, V](opts)
	return &LruCache[K, V]{
		c,
	}
}
