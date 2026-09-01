// Package revocation 定义 JWT 撤销的存储契约。
//
// Checker/Revoker 拆成两个小接口而非一个大接口：中间件只依赖 Checker，
// 未来新增能力（如批量撤销）可以再加独立小接口，不破坏现有实现
// （optional interface pattern）。
package revocation

import (
	"context"
	"time"
)

// Checker 检查 JTI 是否已被撤销。
type Checker interface {
	// IsRevoked 返回 jti 对应的 token 是否已被撤销。
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

// Revoker 撤销指定 JTI。
type Revoker interface {
	// Revoke 撤销 jti，ttl 应设置为该 token 的剩余有效期，
	// 过期后实现方应自动清除撤销记录，避免撤销表无限增长。
	Revoke(ctx context.Context, jti string, ttl time.Duration) error
}

// Store 同时具备撤销与检查能力的完整存储接口。
type Store interface {
	Checker
	Revoker
}
