package captcha

import (
	"gitee.com/byx_darwin/go-tools/tools/cache"
	"gitee.com/byx_darwin/go-tools/tools/cache/core"
	"time"
)

func NewCacheStore(length int) *CacheStore {
	return &CacheStore{
		Expiration: time.Minute * 5,
		PreKey:     "CAPTCHA_",
		Cache:      core.NewFifoCache[string, string](length),
	}

}

type CacheStore struct {
	Expiration time.Duration
	PreKey     string
	Cache      cache.Cache[string, string]
}

func (c *CacheStore) Set(id string, value string) error {
	c.Cache.SetWithExp(c.PreKey+id, value, c.Expiration)
	return nil
}
func (c *CacheStore) Get(key string, clear bool) string {
	val, exist := c.Cache.Get(key)
	if !exist {
		return ""
	}
	if clear {
		c.Cache.Delete(key)
	}
	return val
}

func (c *CacheStore) Verify(id, answer string, clear bool) bool {
	key := c.PreKey + id
	v := c.Get(key, clear)
	return v == answer
}
