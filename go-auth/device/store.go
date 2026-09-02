package device

import "context"

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
