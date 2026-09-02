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

// Store 接口扩展约定：
//
// Go 接口没有"默认方法"机制，直接为 Store 添加新方法会立即破坏所有下游实现
// （go-middleware 的 Redis/Memory 实现）。因此后续新增能力（如批量操作、TTL 续期）
// 必须通过"新增独立小接口 + 可选类型断言"扩展，而不是修改 Store 本身的方法集。
// 参见 TTLRefresher 作为示例。

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

// TTLRefresher 是 Store 的可选扩展接口，用于续期会话 TTL 而不更新会话数据本身。
// 存储实现可选择性支持该接口；调用方应使用类型断言检测：
//
//	if refresher, ok := store.(session.TTLRefresher); ok {
//	    err := refresher.RefreshTTL(ctx, sessionID, ttl)
//	}
type TTLRefresher interface {
	// RefreshTTL 续期指定 sessionID 的过期时间，不修改会话数据本身。
	RefreshTTL(ctx context.Context, sessionID string, ttl time.Duration) error
}
