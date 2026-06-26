package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/byx-darwin/go-tools/go-auth/session"
	"github.com/redis/go-redis/v9"
)

// compile-time interface check.
var _ session.Store = (*RedisSessionStore)(nil)

// sessionData 是 Session 的 JSON 序列化结构。
type sessionData struct {
	ID        string         `json:"id"`
	UserUUID  string         `json:"user_uuid"`
	Data      map[string]any `json:"data,omitempty"`
	ExpiresAt time.Time      `json:"expires_at"`
}

// RedisSessionStore 基于 Redis 的 Session 存储实现。
//
// 使用 Redis String 存储 JSON 序列化的 Session 数据，支持 TTL 自动过期。
// Redis key 格式: {prefix}session:{sessionID}
type RedisSessionStore struct {
	client redis.UniversalClient
	ttl    time.Duration
	prefix string
}

// NewRedisSessionStore 创建 Redis Session 存储。
//
// 默认配置：
//   - sessionTTL: 30 分钟
//   - keyPrefix: ""
func NewRedisSessionStore(client redis.UniversalClient, opts ...Option) *RedisSessionStore {
	cfg := applyDefaults(opts)
	return &RedisSessionStore{
		client: client,
		ttl:    cfg.sessionTTL,
		prefix: cfg.keyPrefix,
	}
}

// sessionKey 构建 Redis key。
func (s *RedisSessionStore) sessionKey(sessionID string) string {
	return s.prefix + "session:" + sessionID
}

// Get 根据 sessionID 获取会话。会话不存在返回 nil, nil。
func (s *RedisSessionStore) Get(ctx context.Context, sessionID string) (*session.Session, error) {
	data, err := s.client.Get(ctx, s.sessionKey(sessionID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("session get: %w", err)
	}

	var sd sessionData
	if err = json.Unmarshal(data, &sd); err != nil {
		return nil, fmt.Errorf("session unmarshal: %w", err)
	}

	return &session.Session{
		ID:        sd.ID,
		UserUUID:  sd.UserUUID,
		Data:      sd.Data,
		ExpiresAt: sd.ExpiresAt,
	}, nil
}

// Save 保存会话到 Redis。
func (s *RedisSessionStore) Save(ctx context.Context, sess *session.Session) error {
	sd := sessionData{
		ID:        sess.ID,
		UserUUID:  sess.UserUUID,
		Data:      sess.Data,
		ExpiresAt: sess.ExpiresAt,
	}

	data, err := json.Marshal(sd)
	if err != nil {
		return fmt.Errorf("session marshal: %w", err)
	}

	if err = s.client.Set(ctx, s.sessionKey(sess.ID), data, s.ttl).Err(); err != nil {
		return fmt.Errorf("session save: %w", err)
	}

	return nil
}

// Delete 删除指定 sessionID 的会话。
func (s *RedisSessionStore) Delete(ctx context.Context, sessionID string) error {
	if err := s.client.Del(ctx, s.sessionKey(sessionID)).Err(); err != nil {
		return fmt.Errorf("session delete: %w", err)
	}
	return nil
}

// Exists 检查指定 sessionID 的会话是否存在。
func (s *RedisSessionStore) Exists(ctx context.Context, sessionID string) (bool, error) {
	n, err := s.client.Exists(ctx, s.sessionKey(sessionID)).Result()
	if err != nil {
		return false, fmt.Errorf("session exists: %w", err)
	}
	return n > 0, nil
}
