# go-auth 模块设计

**日期：** 2026-06-26  
**状态：** 草案

## 1. 背景与目标

go-tools 当前缺少统一的认证能力。ncgo 脚手架生成的项目需要统一处理认证，go-tools 应提供一套标准实现。

**需求：**
- API 服务：无状态认证（JWT）
- APP 端：支持单设备/多设备登录限制
- 内部 Web 后台：前后端分离，统一使用 JWT

**目标：**
- 新增 `go-auth` 模块，提供认证工具层 + 接口约定
- 不包含登录流程、用户管理、RBAC（由业务层实现）
- 预留扩展能力（Session、OAuth2 等）

## 2. 架构决策

### 2.1 模块架构

新增第 4 个模块 `go-auth`，依赖关系：

```
go-common          ← 纯工具（crypto、log、error、timeutil 等）
    ↑
go-auth            ← 认证工具（JWT 泛型、Session/Device 接口，依赖 go-common）
    ↑
go-middleware      ← 中间件客户端（Redis 设备会话实现，依赖 go-common + go-auth）
    ↑
go-framework       ← 框架适配（Hertz/Kitex JWT 中间件，依赖 go-common + go-auth + go-middleware）
```

### 2.2 认证方式

| 场景 | 方案 | 说明 |
|------|------|------|
| API 服务 | JWT | 无状态，前后端分离标准方案 |
| APP 端 | JWT + 设备会话管理 | 支持立即踢出、限制 N 个设备 |
| 内部 Web 后台 | JWT（与 API 统一） | 前后端分离，不引入 Session/Cookie |
| Session | 预留接口，暂不实现 | 未来如需服务端渲染等场景可扩展 |

### 2.3 JWT 设计

**泛型 `Sign[T]` / `Verify[T]`**，业务方完全自定义 Claims 结构：

```go
// 业务方定义
type UserClaims struct {
    UserUUID string `json:"user_uuid"`
    DeviceID string `json:"device_id,omitempty"`
    Role     string `json:"role,omitempty"`
    jwt.RegisteredClaims
}

// 签发
token, err := jwt.Sign(UserClaims{...}, secret)

// 验证
claims, err := jwt.Verify[UserClaims](token, secret)
```

**理由：** JWT payload 本质是 JSON，泛型让业务方自由扩展字段，go-auth 不绑定具体业务结构。

### 2.4 设备会话管理

**需求：** APP 端限制设备数量，新设备登录后旧设备立即失效。

**方案：** JWT + Redis 设备会话表

- JWT claims 包含 `user_uuid`、`device_id`、`jti`（JWT ID）
- Redis Hash 存储：`device:{user_uuid}` → `{device_id: jti}`
- 每次请求：验证 JWT 签名 + 检查 Redis 中 jti 是否匹配
- 支持限制 N 个设备（可配置），超出时踢出最旧设备

**数据结构：**
```
Key:    device:{user_uuid}
Type:   Hash
Fields: {device_id → jti}
TTL:    30 days
```

## 3. 模块设计

### 3.1 go-auth 模块结构

```
go-auth/
├── jwt/
│   ├── token.go       → Sign[T], Verify[T], Refresh[T]
│   └── options.go     → WithExpiration, WithIssuer 等
├── session/
│   ├── store.go       → Store 接口定义（预留）
│   └── session.go     → Session 结构定义
├── device/
│   ├── store.go       → Store 接口定义
│   └── model.go       → Device 结构
└── error/
    └── error.go       → 认证错误码 40000-40099
```

### 3.2 go-auth/jwt — 泛型 JWT 工具

```go
package jwt

import "github.com/golang-jwt/jwt/v5"

// Option 配置选项。
type Option func(*config)

// WithExpiration 设置过期时间。
func WithExpiration(d time.Duration) Option

// WithIssuer 设置签发者。
func WithIssuer(issuer string) Option

// Sign 签发 JWT，支持任意 Claims 类型。
func Sign[T any](claims T, secret []byte, opts ...Option) (string, error)

// Verify 验证 JWT，返回指定类型。
func Verify[T any](token string, secret []byte) (*T, error)

// Refresh 刷新 JWT（延长过期时间，保留原有 claims）。
func Refresh[T any](token string, secret []byte, opts ...Option) (string, error)
```

### 3.3 go-auth/session — Session Store 接口（预留）

```go
package session

// Session 会话信息。
type Session struct {
    ID        string
    UserUUID  string
    Data      map[string]any
    ExpiresAt time.Time
}

// Store Session 存储接口。
type Store interface {
    Get(ctx context.Context, sessionID string) (*Session, error)
    Save(ctx context.Context, session *Session) error
    Delete(ctx context.Context, sessionID string) error
    Exists(ctx context.Context, sessionID string) (bool, error)
}
```

### 3.4 go-auth/device — 设备会话 Store 接口

```go
package device

// Device 设备信息。
type Device struct {
    DeviceID  string
    JTI       string    // JWT ID
    UserUUID  string
    CreatedAt time.Time
}

// Store 设备会话存储接口。
type Store interface {
    // AddDevice 添加设备，返回被踢出的设备列表。
    AddDevice(ctx context.Context, userUUID, deviceID, jti string, maxDevices int) ([]Device, error)

    // CheckDevice 检查设备是否有效（jti 是否匹配）。
    CheckDevice(ctx context.Context, userUUID, deviceID, jti string) (bool, error)

    // RemoveDevice 移除指定设备。
    RemoveDevice(ctx context.Context, userUUID, deviceID string) error

    // RemoveAllDevices 移除用户所有设备（全局登出）。
    RemoveAllDevices(ctx context.Context, userUUID string) error

    // ListDevices 列出用户所有设备。
    ListDevices(ctx context.Context, userUUID string) ([]Device, error)
}
```

### 3.5 go-auth/error — 认证错误码

```go
package autherror

// 认证错误码 40000-40099。
const (
    CodeTokenInvalid   = 40001 // Token 无效
    CodeTokenExpired   = 40002 // Token 已过期
    CodeTokenRevoked   = 40003 // Token 已撤销
    CodeDeviceKicked   = 40004 // 设备已被踢出
    CodeSessionInvalid = 40005 // Session 无效
    CodeSessionExpired = 40006 // Session 已过期
)
```

### 3.6 go-middleware/auth — Redis 实现

```go
package auth

// RedisDeviceStore Redis 设备会话存储。
type RedisDeviceStore struct { ... }

// NewRedisDeviceStore 创建 Redis 设备存储。
func NewRedisDeviceStore(client redis.UniversalClient, opts ...Option) *RedisDeviceStore

// MemoryDeviceStore 内存设备会话存储（开发/测试用）。
type MemoryDeviceStore struct { ... }

// NewMemoryDeviceStore 创建内存设备存储。
func NewMemoryDeviceStore(opts ...Option) *MemoryDeviceStore
```

**Redis 数据结构：**
```
Key:    device:{user_uuid}
Type:   Hash
Fields: {device_id → jti}
TTL:    30 days
```

### 3.7 go-framework/hertz/middleware — Hertz 中间件

```go
package middleware

// JWTAuth JWT 认证中间件。
// 从 Authorization 头解析 Bearer Token，验证签名，将 claims 注入 context。
func JWTAuth[T any](secret []byte, opts ...Option) app.HandlerFunc

// DeviceAuth 设备会话检查中间件。
// 需配合 JWTAuth 使用，检查设备 jti 是否有效。
func DeviceAuth(store device.Store, maxDevices int) app.HandlerFunc
```

**使用示例：**

```go
import (
    "github.com/byx-darwin/go-tools/go-auth/jwt"
    "github.com/byx-darwin/go-tools/go-auth/device"
    "github.com/byx-darwin/go-tools/go-middleware/auth"
    hertzmw "github.com/byx-darwin/go-tools/go-framework/hertz/middleware"
)

// 业务方 Claims
type UserClaims struct {
    UserUUID string `json:"user_uuid"`
    DeviceID string `json:"device_id,omitempty"`
    Role     string `json:"role,omitempty"`
    jwt.RegisteredClaims
}

// 初始化
redisClient, close, _ := redis.NewUniversalClient(ctx, &redis.Config{...})
defer close()
deviceStore := auth.NewRedisDeviceStore(redisClient)

// Hertz 路由
r := hertz.Default()
r.Use(hertzmw.JWTAuth[UserClaims](jwtSecret))
r.Use(hertzmw.DeviceAuth(deviceStore, 5))  // 最多 5 个设备

// 登录接口
r.POST("/login", func(ctx context.Context, c *app.RequestContext) {
    claims := UserClaims{
        UserUUID: user.UUID,
        DeviceID: req.DeviceID,
        Role:     user.Role,
    }
    token, _ := jwt.Sign(claims, jwtSecret, jwt.WithExpiration(24*time.Hour))
    deviceStore.AddDevice(ctx, user.UUID, req.DeviceID, claims.ID, 5)
    c.JSON(200, H{"token": token})
})
```

## 4. 依赖关系

| 模块 | 依赖 | 说明 |
|------|------|------|
| `go-auth` | `go-common` | 使用 error、log 等工具 |
| `go-middleware` | `go-common`, `go-auth` | 提供 Redis 设备会话实现 |
| `go-framework` | `go-common`, `go-auth`, `go-middleware` | 提供 Hertz/Kitex 中间件 |

## 5. 错误码范围

```
go-auth: 40000-40099（认证相关错误）
```

## 6. 后续扩展

- **Session Store Redis 实现**：如需服务端渲染等场景，在 go-middleware 中实现
- **OAuth2 / OIDC**：在 go-auth 中扩展 OAuth2 客户端和服务端支持
- **多因素认证（MFA）**：在 go-auth 中扩展 TOTP 等支持
- **Kitex 中间件**：RPC 服务的 JWT 认证中间件
