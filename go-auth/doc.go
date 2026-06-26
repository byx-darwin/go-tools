// Package auth 提供统一的认证工具和接口定义。
//
// go-auth 是 go-tools 的第 4 个模块，提供：
//   - jwt: JWT 泛型签发/验证/刷新工具
//   - session: Session 存储接口
//   - device: 设备会话管理接口
//   - error: 认证错误码
//
// 依赖关系：
//
//	go-common → go-auth → go-middleware → go-framework
package auth
