// Package session 提供会话存储的接口定义。
//
// Session 结构体和 Store 接口构成了会话存储的契约，支持
// go-middleware 的 Redis/Memory 实现和 go-framework 的 Session 中间件。
package session

import (
	"context"
	"time"
)

// Session 会话信息。
type Session struct {
	ID        string
	UserUUID  string
	Data      map[string]any
	ExpiresAt time.Time
}

// Store Session 存储接口。
type Store interface {
	// Get 根据 sessionID 获取会话。会话不存在返回 nil, nil。
	Get(ctx context.Context, sessionID string) (*Session, error)

	// Save 保存会话到存储。
	Save(ctx context.Context, session *Session) error

	// Delete 删除指定 sessionID 的会话。
	Delete(ctx context.Context, sessionID string) error

	// Exists 检查指定 sessionID 的会话是否存在。
	Exists(ctx context.Context, sessionID string) (bool, error)
}
