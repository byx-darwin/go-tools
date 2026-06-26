// Package device 提供设备会话管理的接口定义。
//
// Device 结构体和 Store 接口构成了设备会话管理的契约，支持
// 多设备登录控制和挤踢策略。go-middleware 的 Redis/Memory 实现
// 将基于此接口提供具体存储实现。
package device

import "time"

// Device 设备会话信息。
type Device struct {
	DeviceID  string
	JTI       string // JWT ID，与 JWT Token 关联
	UserUUID  string
	CreatedAt time.Time
}
