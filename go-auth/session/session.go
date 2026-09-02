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
//
// 实现方必须满足以下安全要求：
//   - SessionID 必须使用 crypto/rand（或等价的密码学安全随机数源）生成，长度不少于 128 bit，
//     不得使用可预测的序列号、时间戳或非密码学安全的伪随机数生成器。
//   - 用户登录成功后必须生成新的 SessionID 并使旧 SessionID 失效，防止会话固定
//     （session fixation）攻击。
//   - 已过期的会话数据必须被清理（通过存储介质的 TTL 机制，或后台 GC 任务），
//     不得无限期保留，避免内存/存储泄漏。
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
