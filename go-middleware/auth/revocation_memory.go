package auth

import (
	"context"
	"time"

	"github.com/samber/hot"
	"github.com/samber/oops"

	"github.com/byx-darwin/go-tools/go-auth/revocation"
)

// compile-time interface check.
var _ revocation.Store = (*MemoryRevocationStore)(nil)

// MemoryRevocationStore 基于内存的 Token 撤销存储实现。
//
// 使用 samber/hot 缓存存储已撤销的 JTI，TTL 由调用方在 Revoke 时显式传入
// （应设置为该 token 的剩余有效期），过期后自动从缓存中清除，避免撤销表无限
// 增长。适用于开发和测试环境，不适合生产使用。
type MemoryRevocationStore struct {
	cache *hot.HotCache[string, struct{}]
}

// NewMemoryRevocationStore 创建内存撤销存储。
//
// 默认配置：
//   - cacheSize: 1024
func NewMemoryRevocationStore(opts ...Option) *MemoryRevocationStore {
	cfg := applyDefaults(opts)
	return &MemoryRevocationStore{
		cache: hot.NewHotCache[string, struct{}](hot.LRU, cfg.cacheSize).Build(),
	}
}

// Revoke 撤销指定 jti，ttl 应设置为该 token 的剩余有效期。
// ttl <= 0 视为无效撤销请求（token 已过期，撤销无意义），不写入，返回 nil。
func (s *MemoryRevocationStore) Revoke(_ context.Context, jti string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	s.cache.SetWithTTL(jti, struct{}{}, ttl)
	return nil
}

// IsRevoked 检查指定 jti 是否已被撤销。
func (s *MemoryRevocationStore) IsRevoked(_ context.Context, jti string) (bool, error) {
	_, ok, err := s.cache.Get(jti)
	if err != nil {
		return false, oops.Wrapf(err, "revocation check")
	}
	return ok, nil
}
