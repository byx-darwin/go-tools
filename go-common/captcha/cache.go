package captcha

import (
	"time"

	"gitee.com/byx_darwin/go-tools/go-common/cache"
)

func NewCacheStore(length int) *CacheStore {
	return &CacheStore{
		Expiration: time.Minute * 5,
		PreKey:     "CAPTCHA_",
		Cache:      cache.New[string, string](cache.FIFO, length).Build(),
	}
}

type CacheStore struct {
	Expiration time.Duration
	PreKey     string
	Cache      *cache.HotCache[string, string]
}

func (c *CacheStore) Set(id string, value string) error {
	c.Cache.SetWithTTL(c.PreKey+id, value, c.Expiration)
	return nil
}

func (c *CacheStore) Get(key string, clear bool) string {
	val, ok, _ := c.Cache.Get(key)
	if !ok {
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
