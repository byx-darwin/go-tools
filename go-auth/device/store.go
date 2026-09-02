package device

import (
	"context"
	"time"
)

// Store 接口扩展约定：
//
// Go 接口没有"默认方法"机制，直接为 Store 添加新方法会立即破坏所有下游实现
// （go-middleware 的 Redis/Memory 实现）。因此后续新增能力（如批量操作、TTL 续期）
// 必须通过"新增独立小接口 + 可选类型断言"扩展，而不是修改 Store 本身的方法集。
// 参见 TTLRefresher 作为示例。

// Store 设备会话存储接口。
//
// Store 定义了设备会话的增删查操作。当用户登录新设备且超过限制时，
// AddDevice 会返回被踢出的旧设备列表。
//
// 实现方必须满足以下安全要求：
//   - AddDevice（读取当前设备数、判断是否超限、踢出最旧设备、写入新设备）等涉及
//     "读-判断-写" 的操作必须保证并发原子性，防止并发登录场景下超过 maxDevices 限制
//     或产生脏写。
//   - 已失效或用户已登出的设备会话数据必须被清理（通过存储介质的 TTL 机制，
//     或后台 GC 任务），不得无限期保留，避免内存/存储泄漏。
type Store interface {
	// AddDevice 注册新设备并返回被踢出的设备。
	// maxDevices 限制该用户允许的最大设备数，当超过限制时最旧的设备会被踢出。
	// 返回值是被踢出的设备列表，列表为空表示没有设备被踢出。
	AddDevice(ctx context.Context, userUUID, deviceID, jti string, maxDevices int) ([]Device, error)

	// CheckDevice 检查指定设备会话是否有效。
	// 返回 true 表示该设备会话有效。
	CheckDevice(ctx context.Context, userUUID, deviceID, jti string) (bool, error)

	// RemoveDevice 移除指定设备的会话。
	RemoveDevice(ctx context.Context, userUUID, deviceID string) error

	// RemoveAllDevices 移除用户的所有设备会话（比如修改密码后）。
	RemoveAllDevices(ctx context.Context, userUUID string) error

	// ListDevices 列出用户的所有已注册设备。
	ListDevices(ctx context.Context, userUUID string) ([]Device, error)
}

// TTLRefresher 是 Store 的可选扩展接口，用于续期设备会话的过期时间。
// 存储实现可选择性支持该接口；调用方应使用类型断言检测：
//
//	if refresher, ok := store.(device.TTLRefresher); ok {
//	    err := refresher.RefreshTTL(ctx, userUUID, deviceID, ttl)
//	}
type TTLRefresher interface {
	// RefreshTTL 续期指定用户设备会话的过期时间。
	RefreshTTL(ctx context.Context, userUUID, deviceID string, ttl time.Duration) error
}
