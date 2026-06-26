package auth

import (
	"context"
	"time"

	"github.com/byx-darwin/go-tools/go-auth/session"
	"github.com/samber/hot"
	"github.com/samber/oops"
)

// compile-time interface check.
var _ session.Store = (*MemorySessionStore)(nil)

// MemorySessionStore 基于内存的 Session 存储实现。
//
// 使用 samber/hot 缓存，支持 TTL 自动过期，适用于开发和测试环境。
// 不适合生产环境使用（进程重启后数据丢失）。
type MemorySessionStore struct {
	cache *hot.HotCache[string, *session.Session]
	ttl   time.Duration
}

// NewMemorySessionStore 创建内存 Session 存储。
//
// 默认配置：
//   - sessionTTL: 30 分钟
//   - cacheSize: 1024
func NewMemorySessionStore(opts ...Option) *MemorySessionStore {
	cfg := applyDefaults(opts)
	return &MemorySessionStore{
		cache: hot.NewHotCache[string, *session.Session](hot.LRU, cfg.cacheSize).Build(),
		ttl:   cfg.sessionTTL,
	}
}

// Get 根据 sessionID 获取会话。会话不存在返回 nil, nil。
func (s *MemorySessionStore) Get(_ context.Context, sessionID string) (*session.Session, error) {
	sess, ok, err := s.cache.Get(sessionID)
	if err != nil {
		return nil, oops.Wrapf(err, "session get")
	}
	if !ok {
		return nil, nil
	}
	return sess, nil
}

// Save 保存会话到存储。
func (s *MemorySessionStore) Save(_ context.Context, sess *session.Session) error {
	s.cache.SetWithTTL(sess.ID, sess, s.ttl)
	return nil
}

// Delete 删除指定 sessionID 的会话。
func (s *MemorySessionStore) Delete(_ context.Context, sessionID string) error {
	s.cache.Delete(sessionID)
	return nil
}

// Exists 检查指定 sessionID 的会话是否存在。
func (s *MemorySessionStore) Exists(_ context.Context, sessionID string) (bool, error) {
	_, ok, err := s.cache.Get(sessionID)
	if err != nil {
		return false, oops.Wrapf(err, "session exists")
	}
	return ok, nil
}
