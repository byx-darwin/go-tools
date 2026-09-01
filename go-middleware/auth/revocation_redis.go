package auth

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/samber/oops"

	"github.com/byx-darwin/go-tools/go-auth/revocation"
)

// compile-time interface check.
var _ revocation.Store = (*RedisRevocationStore)(nil)

// RedisRevocationStore 基于 Redis 的 Token 撤销存储实现。
//
// 使用 Redis String 存储撤销标记：
//   - Key: {prefix}revoked:{jti}
//   - Value: ""（存在即表示已撤销）
//
// 撤销记录的 TTL 由调用方在 Revoke 时显式传入（应设置为该 token 的
// 剩余有效期），过期后 Redis 自动清除，避免撤销表无限增长。
type RedisRevocationStore struct {
	client redis.UniversalClient
	prefix string
}

// NewRedisRevocationStore 创建 Redis 撤销存储。
//
// 默认配置：
//   - keyPrefix: ""
func NewRedisRevocationStore(client redis.UniversalClient, opts ...Option) *RedisRevocationStore {
	cfg := applyDefaults(opts)
	return &RedisRevocationStore{
		client: client,
		prefix: cfg.keyPrefix,
	}
}

// revokedKey 构建 Redis key。
func (s *RedisRevocationStore) revokedKey(jti string) string {
	return s.prefix + "revoked:" + jti
}

// Revoke 撤销指定 jti，ttl 应设置为该 token 的剩余有效期。
// ttl <= 0 视为无效撤销请求（token 已过期，撤销无意义），不写入，返回 nil。
func (s *RedisRevocationStore) Revoke(ctx context.Context, jti string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}

	if err := s.client.Set(ctx, s.revokedKey(jti), "", ttl).Err(); err != nil {
		return oops.Wrapf(err, "revocation revoke")
	}

	return nil
}

// IsRevoked 检查指定 jti 是否已被撤销。
func (s *RedisRevocationStore) IsRevoked(ctx context.Context, jti string) (bool, error) {
	_, err := s.client.Get(ctx, s.revokedKey(jti)).Result()
	if err != nil {
		if err == redis.Nil { //nolint:errorlint // go-redis 约定 redis.Nil 为哨兵值，用 == 比较
			return false, nil
		}
		return false, oops.Wrapf(err, "revocation check")
	}

	return true, nil
}
