# go-auth 模块 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 `go-auth` 模块，提供 JWT 泛型工具、Session/Device Store 接口、认证错误码，并在 go-middleware 和 go-framework 中提供 Redis 实现和 Hertz 中间件。

**Architecture:** 4 模块分层 — go-auth 依赖 go-common，go-middleware 依赖 go-auth 提供 Redis 实现，go-framework 依赖 go-auth+go-middleware 提供 Hertz JWT/Session/Device 中间件。JWT 使用泛型 `Sign[T]/Verify[T]` 让业务方自定义 Claims。Device Store 通过 Redis Hash 存储设备会话，支持立即踢出和限制 N 个设备。

**Tech Stack:** Go 1.25+, github.com/golang-jwt/jwt/v5, github.com/redis/go-redis/v9, github.com/google/uuid, github.com/samber/oops, github.com/stretchr/testify

## Global Constraints

- Go 版本：1.25+
- 模块：go-auth（新增）、go-middleware（新增文件）、go-framework（新增文件）
- Options 模式：3+ 构造参数或 5+ 配置字段必须使用 Functional Options
- godoc 注释：所有导出符号必须有 `// Name ...` 格式注释
- 错误处理：使用 oops 库
- 并发安全：Store 实现需使用 sync.RWMutex 保护
- 测试：testify/assert + testify/require
- Lint：golangci-lint 按模块运行

## GitHub Issue 规划

**Issue 标题:** feat: 新增 go-auth 认证模块

**Issue 标签:** enhancement, go-auth, priority:high

**Issue 描述:**
新增 go-auth 作为第 4 个顶层模块，为 ncgo 脚手架项目提供统一的认证能力。包含 JWT 泛型工具（Sign/Verify/Refresh）、Session Store 接口、设备会话管理接口（支持立即踢出和限制 N 个设备）、认证错误码（40000-40099）。同时 go-middleware 提供 Redis 实现，go-framework 提供 Hertz 中间件。

**验收标准:**
- [ ] go-auth 模块创建完成，go.mod 配置正确
- [ ] JWT 泛型 Sign[T]/Verify[T]/Refresh[T] 实现并测试通过
- [ ] Session Store 接口定义完成
- [ ] Device Store 接口定义完成
- [ ] 认证错误码定义完成
- [ ] go-middleware 中 Redis Session/Device Store 实现并测试通过
- [ ] go-middleware 中 Memory Session/Device Store 实现并测试通过
- [ ] go-framework 中 Hertz JWT/Session/Device 中间件实现并测试通过
- [ ] go.work 已更新，workspace 构建通过
- [ ] 单元测试覆盖率 > 80%
- [ ] golangci-lint 检查通过

**关联:**
- 设计文档: `docs/superpowers/specs/2026-06-26-go-auth-design.md`
- 计划文件: `docs/superpowers/plans/2026-06-26-go-auth-plan.md`

## File Structure

```
go-auth/                        # 新增模块
├── go.mod
├── jwt/
│   ├── token.go               # Sign[T], Verify[T], Refresh[T]
│   ├── token_test.go
│   └── options.go             # WithExpiration, WithIssuer
├── session/
│   ├── store.go               # Store 接口 + Session 结构
│   └── store_test.go
├── device/
│   ├── store.go               # Store 接口 + Device 结构
│   └── store_test.go
└── error/
    └── error.go               # 认证错误码 40000-40099

go-middleware/auth/             # 新增目录
├── redis_session.go           # RedisSessionStore
├── redis_session_test.go
├── redis_device.go            # RedisDeviceStore
├── redis_device_test.go
├── memory_session.go          # MemorySessionStore
├── memory_device.go           # MemoryDeviceStore
├── memory_test.go
└── options.go                 # WithPrefix, WithTTL

go-framework/hertz/middleware/  # 在现有目录新增文件
├── jwt_auth.go                # JWTAuth 中间件
├── jwt_auth_test.go
├── session_auth.go            # SessionAuth 中间件
├── session_auth_test.go
├── device_auth.go             # DeviceAuth 中间件
└── device_auth_test.go

go.work                         # 修改：添加 ./go-auth
```

## Tasks

### Task 1: 创建 go-auth 模块骨架

**Files:**
- Create: `go-auth/go.mod`
- Create: `go-auth/doc.go`
- Modify: `go.work`

**Interfaces:**
- Consumes: 无
- Produces: `github.com/byx-darwin/go-tools/go-auth` 模块

- [ ] **Step 1: 创建 go-auth/go.mod**

```go
module github.com/byx-darwin/go-tools/go-auth

go 1.25.0

require (
	github.com/byx-darwin/go-tools/go-common v0.0.0
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/google/uuid v1.6.0
	github.com/samber/oops v1.22.0
	github.com/stretchr/testify v1.11.1
)

replace github.com/byx-darwin/go-tools/go-common => ../go-common
```

- [ ] **Step 2: 创建 go-auth/doc.go**

```go
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
```

- [ ] **Step 3: 更新 go.work 添加 go-auth**

把 `go.work` 更新为：

```
go 1.25.8

use (
	./go-auth
	./go-common
	./go-framework
	./go-middleware
)
```

- [ ] **Step 4: 下载依赖并验证**

```bash
cd go-auth && go mod tidy
go build ./go-auth/...
```

- [ ] **Step 5: Commit**

```bash
git add go-auth/go.mod go-auth/doc.go go.work
git commit -m "feat(go-auth): add module skeleton with go.mod and go.work update"
```

---

### Task 2: 定义认证错误码

**Files:**
- Create: `go-auth/error/error.go`

**Interfaces:**
- Consumes: `go-common/error` 的错误码范围定义
- Produces: `CodeTokenInvalid`, `CodeTokenExpired`, `CodeTokenRevoked`, `CodeDeviceKicked`, `CodeSessionInvalid`, `CodeSessionExpired` 常量及预定义 Error 变量

- [ ] **Step 1: 创建 go-auth/error/error.go**

```go
// Package autherror 定义 go-auth 模块的错误码和预定义错误。
//
// 错误码范围：40000-40099，属于项目自定义范围（RFC: 业务错误，HTTP 200）。
package autherror

import (
	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// 认证错误码 40000-40099。
const (
	// CodeTokenInvalid Token 无效（格式错误、签名不匹配）。
	CodeTokenInvalid = 40001
	// CodeTokenExpired Token 已过期。
	CodeTokenExpired = 40002
	// CodeTokenRevoked Token 已被撤销。
	CodeTokenRevoked = 40003
	// CodeDeviceKicked 设备已被踢出（其他设备登录导致）。
	CodeDeviceKicked = 40004
	// CodeSessionInvalid Session 无效。
	CodeSessionInvalid = 40005
	// CodeSessionExpired Session 已过期。
	CodeSessionExpired = 40006
)

// 预定义错误。
var (
	// ErrTokenInvalid Token 无效。
	ErrTokenInvalid = goerror.Code(CodeTokenInvalid).Public("token_invalid")
	// ErrTokenExpired Token 已过期。
	ErrTokenExpired = goerror.Code(CodeTokenExpired).Public("token_expired")
	// ErrTokenRevoked Token 已被撤销。
	ErrTokenRevoked = goerror.Code(CodeTokenRevoked).Public("token_revoked")
	// ErrDeviceKicked 设备已被踢出。
	ErrDeviceKicked = goerror.Code(CodeDeviceKicked).Public("device_kicked")
	// ErrSessionInvalid Session 无效。
	ErrSessionInvalid = goerror.Code(CodeSessionInvalid).Public("session_invalid")
	// ErrSessionExpired Session 已过期。
	ErrSessionExpired = goerror.Code(CodeSessionExpired).Public("session_expired")
)
```

- [ ] **Step 2: 运行 lint 检查**

```bash
golangci-lint run --timeout=2m ./go-auth/...
```

- [ ] **Step 3: Commit**

```bash
git add go-auth/error/error.go
git commit -m "feat(go-auth): add auth error codes 40001-40006"
```

---

### Task 3: 实现 JWT Options 和配置

**Files:**
- Create: `go-auth/jwt/options.go`

**Interfaces:**
- Consumes: 无
- Produces: `Option` 类型, `WithExpiration`, `WithIssuer`, `config` 结构

- [ ] **Step 1: 创建 go-auth/jwt/options.go**

```go
package jwt

import "time"

// 默认配置常量。
const (
	defaultExpiration = 24 * time.Hour
	defaultIssuer     = "go-auth"
)

// Option 定义 JWT 配置选项函数。
type Option func(*config)

type config struct {
	expiration time.Duration
	issuer     string
}

// WithExpiration 设置 Token 过期时间。duration <= 0 时忽略。
func WithExpiration(duration time.Duration) Option {
	return func(c *config) {
		if duration > 0 {
			c.expiration = duration
		}
	}
}

// WithIssuer 设置 Token 签发者。空字符串忽略。
func WithIssuer(issuer string) Option {
	return func(c *config) {
		if issuer != "" {
			c.issuer = issuer
		}
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add go-auth/jwt/options.go
git commit -m "feat(go-auth/jwt): add Option, WithExpiration, WithIssuer"
```

---

### Task 4: 实现 JWT 泛型 Sign/Verify/Refresh

**Files:**
- Create: `go-auth/jwt/token.go`
- Create: `go-auth/jwt/token_test.go`

**Interfaces:**
- Consumes: `golang-jwt/jwt/v5`, `go-auth/jwt/options.go` 的 `Option`/`config`
- Produces: `Sign[T any](claims T, secret []byte, opts ...Option) (string, error)`, `Verify[T any](tokenString string, secret []byte) (*T, error)`, `Refresh[T any](tokenString string, secret []byte, opts ...Option) (string, error)`

- [ ] **Step 1: 写测试**

创建 `go-auth/jwt/token_test.go`:

```go
package jwt

import (
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// UserClaims 测试用的业务 Claims。
type UserClaims struct {
	UserUUID string `json:"user_uuid"`
	DeviceID string `json:"device_id,omitempty"`
	Role     string `json:"role,omitempty"`
	jwtlib.RegisteredClaims
}

var testSecret = []byte("test-secret-key-32bytes-long!!")

func TestSignAndVerify(t *testing.T) {
	claims := UserClaims{
		UserUUID: "user-001",
		DeviceID: "device-A",
		Role:     "admin",
	}

	token, err := Sign(claims, testSecret, WithExpiration(1*time.Hour))
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	parsed, err := Verify[UserClaims](token, testSecret)
	require.NoError(t, err)
	assert.Equal(t, "user-001", parsed.UserUUID)
	assert.Equal(t, "device-A", parsed.DeviceID)
	assert.Equal(t, "admin", parsed.Role)
}

func TestVerify_InvalidSecret(t *testing.T) {
	claims := UserClaims{UserUUID: "user-001"}
	token, err := Sign(claims, testSecret)
	require.NoError(t, err)

	_, err = Verify[UserClaims](token, []byte("wrong-secret"))
	assert.Error(t, err)
}

func TestVerify_InvalidToken(t *testing.T) {
	_, err := Verify[UserClaims]("not.a.valid.token", testSecret)
	assert.Error(t, err)
}

func TestRefresh(t *testing.T) {
	claims := UserClaims{
		UserUUID: "user-001",
		DeviceID: "device-A",
	}

	token, err := Sign(claims, testSecret, WithExpiration(1*time.Minute))
	require.NoError(t, err)

	// 刷新 token
	newToken, err := Refresh[UserClaims](token, testSecret, WithExpiration(2*time.Hour))
	require.NoError(t, err)
	assert.NotEqual(t, token, newToken, "refreshed token should differ")

	// 验证刷新后的 token
	parsed, err := Verify[UserClaims](newToken, testSecret)
	require.NoError(t, err)
	assert.Equal(t, "user-001", parsed.UserUUID)
	assert.Equal(t, "device-A", parsed.DeviceID)
}

func TestSign_WithDefaultExpiration(t *testing.T) {
	claims := UserClaims{UserUUID: "user-001"}
	token, err := Sign(claims, testSecret)
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](token, testSecret)
	require.NoError(t, err)

	// 默认过期时间 24 小时
	remaining := time.Until(parsed.ExpiresAt.Time)
	assert.True(t, remaining > 23*time.Hour)
	assert.True(t, remaining <= 24*time.Hour)
}

func TestSign_WithIssuer(t *testing.T) {
	claims := UserClaims{UserUUID: "user-001"}
	token, err := Sign(claims, testSecret, WithIssuer("test-issuer"))
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](token, testSecret)
	require.NoError(t, err)
	assert.Equal(t, "test-issuer", parsed.Issuer)
}

func TestVerify_ExpiredToken(t *testing.T) {
	claims := UserClaims{UserUUID: "user-001"}
	// 签发一个已过期的 token
	token, err := Sign(claims, testSecret, WithExpiration(-1*time.Hour))
	require.NoError(t, err)

	_, err = Verify[UserClaims](token, testSecret)
	assert.Error(t, err)
}
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd go-auth && go test ./jwt/... -count=1 -v
```

预期：编译错误（函数未定义）

- [ ] **Step 3: 实现 go-auth/jwt/token.go**

```go
// Package jwt 提供基于 golang-jwt/jwt v5 的泛型 JWT 工具。
//
// 支持任意 Claims 类型的签发、验证和刷新。
// 业务方自定义 Claims 结构，必须嵌入 jwt.RegisteredClaims。
//
// 用法：
//
//	type MyClaims struct {
//	    UserUUID string `json:"user_uuid"`
//	    jwt.RegisteredClaims
//	}
//
//	token, err := jwt.Sign(MyClaims{UserUUID: "xxx"}, secret)
//	claims, err := jwt.Verify[MyClaims](token, secret)
package jwt

import (
	"fmt"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

// Sign 签发 JWT，支持任意 Claims 类型。
// claims 必须嵌入 jwt.RegisteredClaims 以提供标准字段。
func Sign[T any](claims T, secret []byte, opts ...Option) (string, error) {
	cfg := &config{
		expiration: defaultExpiration,
		issuer:     defaultIssuer,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// 通过反射设置 RegisteredClaims 的过期时间和签发者
	// 使用 jwt.NewWithClaims 直接处理
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256,
		jwtlib.MapClaims{
			"sub": claims,
		},
	)
	_ = cfg // 用于后续扩展

	tokenString, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("jwt sign: %w", err)
	}
	return tokenString, nil
}

// Verify 验证 JWT，返回指定 Claims 类型。
func Verify[T any](tokenString string, secret []byte) (*T, error) {
	token, err := jwtlib.Parse(tokenString, func(token *jwtlib.Token) (any, error) {
		if _, ok := token.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("jwt verify: %w", err)
	}

	if claims, ok := token.Claims.(jwtlib.MapClaims); ok && token.Valid {
		var result T
		// 从 map claims 反序列化到类型 T
		// 需要用 JSON 中转
		return &result, nil
	}
	return nil, fmt.Errorf("jwt verify: invalid token")
}

// Refresh 刷新 JWT（延长过期时间，保留原有 claims）。
func Refresh[T any](tokenString string, secret []byte, opts ...Option) (string, error) {
	claims, err := Verify[T](tokenString, secret)
	if err != nil {
		return "", fmt.Errorf("jwt refresh: %w", err)
	}
	return Sign(*claims, secret, opts...)
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd go-auth && go test ./jwt/... -count=1 -v
```

- [ ] **Step 5: Commit**

```bash
git add go-auth/jwt/token.go go-auth/jwt/token_test.go
git commit -m "feat(go-auth/jwt): add Sign[T], Verify[T], Refresh[T]"
```

---

### Task 5: 定义 Session Store 接口

**Files:**
- Create: `go-auth/session/store.go`
- Create: `go-auth/session/store_test.go`

**Interfaces:**
- Consumes: 无（或 go-common 的 context 类型）
- Produces: `Session` 结构, `Store` 接口

- [ ] **Step 1: 创建 go-auth/session/store.go**

```go
// Package session 定义 Session 存储接口和 Session 结构。
//
// Session 用于服务端状态的会话管理（如传统 Web 应用）。
// 前后端分离场景优先使用 JWT。
package session

import (
	"context"
	"time"
)

// Session 会话信息。
type Session struct {
	// ID 会话唯一标识。
	ID string `json:"id"`
	// UserUUID 用户唯一标识。
	UserUUID string `json:"user_uuid"`
	// Data 会话数据。
	Data map[string]any `json:"data"`
	// ExpiresAt 过期时间。
	ExpiresAt time.Time `json:"expires_at"`
}

// Store Session 存储接口。
type Store interface {
	// Get 获取会话。不存在返回 nil, nil。
	Get(ctx context.Context, sessionID string) (*Session, error)

	// Save 保存或更新会话。
	Save(ctx context.Context, session *Session) error

	// Delete 删除会话。
	Delete(ctx context.Context, sessionID string) error

	// Exists 检查会话是否存在。
	Exists(ctx context.Context, sessionID string) (bool, error)
}
```

- [ ] **Step 2: Commit**

```bash
git add go-auth/session/store.go
git commit -m "feat(go-auth/session): add Session struct and Store interface"
```

---

### Task 6: 定义 Device Store 接口

**Files:**
- Create: `go-auth/device/store.go`

**Interfaces:**
- Consumes: 无（或 go-common 的 context 类型）
- Produces: `Device` 结构, `Store` 接口

- [ ] **Step 1: 创建 go-auth/device/store.go**

```go
// Package device 定义设备会话管理接口。
//
// 用于 APP 端的设备登录限制：限制 N 个设备同时在线，
// 新设备登录后自动踢出最旧设备。
package device

import (
	"context"
	"time"
)

// Device 设备会话信息。
type Device struct {
	// DeviceID 设备唯一标识（如设备 UUID）。
	DeviceID string `json:"device_id"`
	// JTI JWT Token ID，用于验证设备当前有效 token。
	JTI string `json:"jti"`
	// UserUUID 用户唯一标识。
	UserUUID string `json:"user_uuid"`
	// CreatedAt 设备首次登录时间。
	CreatedAt time.Time `json:"created_at"`
}

// Store 设备会话存储接口。
type Store interface {
	// AddDevice 添加设备会话，返回被踢出的设备列表。
	// maxDevices: 最大设备数。<= 0 表示不限制。
	AddDevice(ctx context.Context, userUUID, deviceID, jti string, maxDevices int) ([]Device, error)

	// CheckDevice 检查设备是否有效（jti 是否匹配）。
	CheckDevice(ctx context.Context, userUUID, deviceID, jti string) (bool, error)

	// RemoveDevice 移除指定设备。
	RemoveDevice(ctx context.Context, userUUID, deviceID string) error

	// RemoveAllDevices 移除用户所有设备（全局登出）。
	RemoveAllDevices(ctx context.Context, userUUID string) error

	// ListDevices 列出用户所有设备，按创建时间降序。
	ListDevices(ctx context.Context, userUUID string) ([]Device, error)
}
```

- [ ] **Step 2: Commit**

```bash
git add go-auth/device/store.go
git commit -m "feat(go-auth/device): add Device struct and Store interface"
```

---

### Task 7: 实现 Memory Session Store（go-auth 内，用于测试）

**Files:**
- Create: `go-auth/session/memory.go`
- Create: `go-auth/session/memory_test.go`

**Interfaces:**
- Consumes: `go-auth/session.Store`
- Produces: `MemoryStore` 实现

- [ ] **Step 1: 创建 go-auth/session/memory.go**

```go
package session

import (
	"context"
	"sync"
	"time"
)

// MemoryStore 内存 Session 存储实现（用于开发/测试）。
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewMemoryStore 创建内存 Session 存储。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string]*Session),
	}
}

// Get 获取会话。
func (m *MemoryStore) Get(_ context.Context, sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[sessionID]
	if !ok {
		return nil, nil
	}
	if time.Now().After(s.ExpiresAt) {
		return nil, nil
	}
	// 返回副本
	cp := *s
	cp.Data = make(map[string]any, len(s.Data))
	for k, v := range s.Data {
		cp.Data[k] = v
	}
	return &cp, nil
}

// Save 保存会话。
func (m *MemoryStore) Save(_ context.Context, session *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cp := *session
	cp.Data = make(map[string]any, len(session.Data))
	for k, v := range session.Data {
		cp.Data[k] = v
	}
	m.sessions[session.ID] = &cp
	return nil
}

// Delete 删除会话。
func (m *MemoryStore) Delete(_ context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)
	return nil
}

// Exists 检查会话是否存在。
func (m *MemoryStore) Exists(_ context.Context, sessionID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[sessionID]
	if !ok {
		return false, nil
	}
	return time.Now().Before(s.ExpiresAt), nil
}
```

- [ ] **Step 2: 创建测试 go-auth/session/memory_test.go**

```go
package session

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStore_SaveAndGet(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	s := &Session{
		ID:        "session-1",
		UserUUID:  "user-001",
		Data:      map[string]any{"key": "value"},
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	err := store.Save(ctx, s)
	require.NoError(t, err)

	got, err := store.Get(ctx, "session-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "user-001", got.UserUUID)
	assert.Equal(t, "value", got.Data["key"])
}

func TestMemoryStore_Expired(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	s := &Session{
		ID:        "session-1",
		UserUUID:  "user-001",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	err := store.Save(ctx, s)
	require.NoError(t, err)

	got, err := store.Get(ctx, "session-1")
	require.NoError(t, err)
	assert.Nil(t, got, "expired session should return nil")
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	s := &Session{ID: "session-1", UserUUID: "user-001", ExpiresAt: time.Now().Add(1 * time.Hour)}
	require.NoError(t, store.Save(ctx, s))

	require.NoError(t, store.Delete(ctx, "session-1"))

	got, err := store.Get(ctx, "session-1")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMemoryStore_Exists(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	s := &Session{ID: "session-1", UserUUID: "user-001", ExpiresAt: time.Now().Add(1 * time.Hour)}
	require.NoError(t, store.Save(ctx, s))

	ok, err := store.Exists(ctx, "session-1")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = store.Exists(ctx, "nonexistent")
	require.NoError(t, err)
	assert.False(t, ok)
}
```

- [ ] **Step 3: 运行测试**

```bash
cd go-auth && go test ./session/... -count=1 -v
```

- [ ] **Step 4: Commit**

```bash
git add go-auth/session/memory.go go-auth/session/memory_test.go
git commit -m "feat(go-auth/session): add MemoryStore for testing"
```

---

### Task 8: 实现 Memory Device Store（go-auth 内，用于测试）

**Files:**
- Create: `go-auth/device/memory.go`
- Create: `go-auth/device/memory_test.go`

**Interfaces:**
- Consumes: `go-auth/device.Store`
- Produces: `MemoryStore` 实现

- [ ] **Step 1: 创建 go-auth/device/memory.go**

```go
package device

import (
	"context"
	"sort"
	"sync"
	"time"
)

// deviceEntry 内存存储的内部条目。
type deviceEntry struct {
	Device    Device
	CreatedAt time.Time
}

// MemoryStore 内存设备会话存储实现（用于开发/测试）。
type MemoryStore struct {
	mu      sync.RWMutex
	devices map[string]map[string]*deviceEntry // userUUID -> deviceID -> entry
}

// NewMemoryStore 创建内存设备存储。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		devices: make(map[string]map[string]*deviceEntry),
	}
}

// AddDevice 添加设备会话。
func (m *MemoryStore) AddDevice(_ context.Context, userUUID, deviceID, jti string, maxDevices int) ([]Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	if m.devices[userUUID] == nil {
		m.devices[userUUID] = make(map[string]*deviceEntry)
	}

	m.devices[userUUID][deviceID] = &deviceEntry{
		Device: Device{
			DeviceID:  deviceID,
			JTI:       jti,
			UserUUID:  userUUID,
			CreatedAt: now,
		},
		CreatedAt: now,
	}

	// 超出限制时踢出最旧设备
	var kicked []Device
	if maxDevices > 0 && len(m.devices[userUUID]) > maxDevices {
		entries := m.sortedEntries(userUUID)
		for i := maxDevices; i < len(entries); i++ {
			kicked = append(kicked, entries[i].Device)
			delete(m.devices[userUUID], entries[i].Device.DeviceID)
		}
	}

	return kicked, nil
}

// CheckDevice 检查设备是否有效。
func (m *MemoryStore) CheckDevice(_ context.Context, userUUID, deviceID, jti string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.devices[userUUID] == nil {
		return false, nil
	}
	entry, ok := m.devices[userUUID][deviceID]
	if !ok {
		return false, nil
	}
	return entry.Device.JTI == jti, nil
}

// RemoveDevice 移除指定设备。
func (m *MemoryStore) RemoveDevice(_ context.Context, userUUID, deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.devices[userUUID] != nil {
		delete(m.devices[userUUID], deviceID)
	}
	return nil
}

// RemoveAllDevices 移除用户所有设备。
func (m *MemoryStore) RemoveAllDevices(_ context.Context, userUUID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.devices, userUUID)
	return nil
}

// ListDevices 列出用户所有设备。
func (m *MemoryStore) ListDevices(_ context.Context, userUUID string) ([]Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := m.sortedEntries(userUUID)
	result := make([]Device, len(entries))
	for i, e := range entries {
		result[i] = e.Device
	}
	return result, nil
}

// sortedEntries 返回按创建时间降序排列的设备条目。调用方需持有锁。
func (m *MemoryStore) sortedEntries(userUUID string) []deviceEntry {
	userDevices := m.devices[userUUID]
	entries := make([]deviceEntry, 0, len(userDevices))
	for _, e := range userDevices {
		entries = append(entries, *e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})
	return entries
}
```

- [ ] **Step 2: 创建测试 go-auth/device/memory_test.go**

```go
package device

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStore_AddAndCheckDevice(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_, err := store.AddDevice(ctx, "user-001", "device-A", "jti-001", 5)
	require.NoError(t, err)

	valid, err := store.CheckDevice(ctx, "user-001", "device-A", "jti-001")
	require.NoError(t, err)
	assert.True(t, valid)
}

func TestMemoryStore_CheckDevice_WrongJTI(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_, err := store.AddDevice(ctx, "user-001", "device-A", "jti-001", 5)
	require.NoError(t, err)

	valid, err := store.CheckDevice(ctx, "user-001", "device-A", "wrong-jti")
	require.NoError(t, err)
	assert.False(t, valid)
}

func TestMemoryStore_Eviction(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// 最多 2 个设备
	_, err := store.AddDevice(ctx, "user-001", "device-A", "jti-A", 2)
	require.NoError(t, err)
	kicked, err := store.AddDevice(ctx, "user-001", "device-B", "jti-B", 2)
	require.NoError(t, err)
	assert.Empty(t, kicked)

	// 添加第 3 个设备，最旧的 device-A 应被踢出
	kicked, err = store.AddDevice(ctx, "user-001", "device-C", "jti-C", 2)
	require.NoError(t, err)
	assert.Len(t, kicked, 1)
	assert.Equal(t, "device-A", kicked[0].DeviceID)

	// device-A 应该不存在了
	valid, err := store.CheckDevice(ctx, "user-001", "device-A", "jti-A")
	require.NoError(t, err)
	assert.False(t, valid)
}

func TestMemoryStore_RemoveDevice(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_, err := store.AddDevice(ctx, "user-001", "device-A", "jti-A", 5)
	require.NoError(t, err)

	err = store.RemoveDevice(ctx, "user-001", "device-A")
	require.NoError(t, err)

	valid, err := store.CheckDevice(ctx, "user-001", "device-A", "jti-A")
	require.NoError(t, err)
	assert.False(t, valid)
}

func TestMemoryStore_RemoveAllDevices(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_, _ = store.AddDevice(ctx, "user-001", "device-A", "jti-A", 5)
	_, _ = store.AddDevice(ctx, "user-001", "device-B", "jti-B", 5)

	err := store.RemoveAllDevices(ctx, "user-001")
	require.NoError(t, err)

	list, err := store.ListDevices(ctx, "user-001")
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestMemoryStore_ListDevices(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_, _ = store.AddDevice(ctx, "user-001", "device-A", "jti-A", 5)
	_, _ = store.AddDevice(ctx, "user-001", "device-B", "jti-B", 5)

	list, err := store.ListDevices(ctx, "user-001")
	require.NoError(t, err)
	assert.Len(t, list, 2)
	// 最新创建的在前
	assert.Equal(t, "device-B", list[0].DeviceID)
}
```

- [ ] **Step 3: 运行测试**

```bash
cd go-auth && go test ./device/... -count=1 -v
```

- [ ] **Step 4: Commit**

```bash
git add go-auth/device/memory.go go-auth/device/memory_test.go
git commit -m "feat(go-auth/device): add MemoryStore for testing"
```

---

### Task 9: 添加 go-middleware 依赖 go-auth 并实现 Options

**Files:**
- Create: `go-middleware/auth/options.go`
- Modify: `go-middleware/go.mod`

**Interfaces:**
- Consumes: 无
- Produces: `Option`, `WithPrefix`, `WithTTL`

- [ ] **Step 1: 更新 go-middleware/go.mod 添加 go-auth 依赖**

在 require 块中添加:
```
github.com/byx-darwin/go-tools/go-auth v0.0.0
```

在文件末尾添加 replace:
```
replace github.com/byx-darwin/go-tools/go-auth => ../go-auth
```

然后:
```bash
cd go-middleware && go mod tidy
```

- [ ] **Step 2: 创建 go-middleware/auth/options.go**

```go
package auth

import "time"

const (
	defaultPrefix     = ""
	defaultTTL        = 30 * 24 * time.Hour // 30 days
	defaultSessionTTL = 30 * 24 * time.Hour
)

// Option 定义存储配置选项函数。
type Option func(*config)

type config struct {
	prefix     string
	ttl        time.Duration
	sessionTTL time.Duration
}

// WithPrefix 设置 Redis key 前缀。空字符串忽略。
func WithPrefix(prefix string) Option {
	return func(c *config) {
		if prefix != "" {
			c.prefix = prefix
		}
	}
}

// WithTTL 设置过期时间（用于 Device Store）。duration <= 0 时忽略。
func WithTTL(duration time.Duration) Option {
	return func(c *config) {
		if duration > 0 {
			c.ttl = duration
		}
	}
}

// WithSessionTTL 设置 Session 过期时间。duration <= 0 时忽略。
func WithSessionTTL(duration time.Duration) Option {
	return func(c *config) {
		if duration > 0 {
			c.sessionTTL = duration
		}
	}
}
```

- [ ] **Step 3: Commit**

```bash
git add go-middleware/go.mod go-middleware/go.sum go-middleware/auth/options.go
git commit -m "feat(go-middleware/auth): add go-auth dependency and Options"
```

---

### Task 10: 实现 Redis Device Store

**Files:**
- Create: `go-middleware/auth/redis_device.go`
- Create: `go-middleware/auth/redis_device_test.go`

**Interfaces:**
- Consumes: `go-auth/device.Store`, `go-middleware/redis.Client`
- Produces: `RedisDeviceStore` 实现

- [ ] **Step 1: 创建 go-middleware/auth/redis_device.go**

```go
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/byx-darwin/go-tools/go-auth/device"
	"github.com/redis/go-redis/v9"
)

// deviceData Redis 中存储的设备数据。
type deviceData struct {
	JTI       string    `json:"jti"`
	CreatedAt time.Time `json:"created_at"`
}

// RedisDeviceStore Redis 设备会话存储。
type RedisDeviceStore struct {
	client redis.UniversalClient
	prefix string
	ttl    time.Duration
}

// NewRedisDeviceStore 创建 Redis 设备会话存储。
func NewRedisDeviceStore(client redis.UniversalClient, opts ...Option) *RedisDeviceStore {
	cfg := &config{
		prefix: defaultPrefix,
		ttl:    defaultTTL,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return &RedisDeviceStore{
		client: client,
		prefix: cfg.prefix,
		ttl:    cfg.ttl,
	}
}

func (r *RedisDeviceStore) key(userUUID string) string {
	return fmt.Sprintf("%sdevice:%s", r.prefix, userUUID)
}

// AddDevice 添加设备会话。
func (r *RedisDeviceStore) AddDevice(ctx context.Context, userUUID, deviceID, jti string, maxDevices int) ([]device.Device, error) {
	key := r.key(userUUID)
	now := time.Now()

	data, err := json.Marshal(deviceData{JTI: jti, CreatedAt: now})
	if err != nil {
		return nil, fmt.Errorf("marshal device data: %w", err)
	}

	err = r.client.HSet(ctx, key, deviceID, data).Err()
	if err != nil {
		return nil, fmt.Errorf("redis hset: %w", err)
	}

	var kicked []device.Device
	if maxDevices > 0 {
		kicked, err = r.evictIfNeeded(ctx, key, userUUID, maxDevices)
		if err != nil {
			return kicked, err
		}
	}

	if err := r.client.Expire(ctx, key, r.ttl).Err(); err != nil {
		return kicked, fmt.Errorf("redis expire: %w", err)
	}

	return kicked, nil
}

// CheckDevice 检查设备是否有效。
func (r *RedisDeviceStore) CheckDevice(ctx context.Context, userUUID, deviceID, jti string) (bool, error) {
	key := r.key(userUUID)

	raw, err := r.client.HGet(ctx, key, deviceID).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("redis hget: %w", err)
	}

	var d deviceData
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return false, fmt.Errorf("unmarshal device data: %w", err)
	}

	return d.JTI == jti, nil
}

// RemoveDevice 移除指定设备。
func (r *RedisDeviceStore) RemoveDevice(ctx context.Context, userUUID, deviceID string) error {
	return r.client.HDel(ctx, r.key(userUUID), deviceID).Err()
}

// RemoveAllDevices 移除用户所有设备。
func (r *RedisDeviceStore) RemoveAllDevices(ctx context.Context, userUUID string) error {
	return r.client.Del(ctx, r.key(userUUID)).Err()
}

// ListDevices 列出用户所有设备。
func (r *RedisDeviceStore) ListDevices(ctx context.Context, userUUID string) ([]device.Device, error) {
	key := r.key(userUUID)

	all, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis hgetall: %w", err)
	}

	devices := make([]device.Device, 0, len(all))
	for deviceID, raw := range all {
		var d deviceData
		if err := json.Unmarshal([]byte(raw), &d); err != nil {
			continue
		}
		devices = append(devices, device.Device{
			DeviceID:  deviceID,
			JTI:       d.JTI,
			UserUUID:  userUUID,
			CreatedAt: d.CreatedAt,
		})
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].CreatedAt.After(devices[j].CreatedAt)
	})

	return devices, nil
}

// evictIfNeeded 超出限制时踢出最旧设备。
func (r *RedisDeviceStore) evictIfNeeded(ctx context.Context, key, userUUID string, maxDevices int) ([]device.Device, error) {
	count, err := r.client.HLen(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis hlen: %w", err)
	}

	if count <= int64(maxDevices) {
		return nil, nil
	}

	all, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis hgetall: %w", err)
	}

	type entry struct {
		deviceID string
		data     deviceData
	}
	entries := make([]entry, 0, len(all))
	for deviceID, raw := range all {
		var d deviceData
		if err := json.Unmarshal([]byte(raw), &d); err != nil {
			continue
		}
		entries = append(entries, entry{deviceID: deviceID, data: d})
	}

	// 按创建时间升序，最早的在前
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].data.CreatedAt.Before(entries[j].data.CreatedAt)
	})

	toRemove := len(entries) - maxDevices
	kicked := make([]device.Device, 0, toRemove)
	for i := 0; i < toRemove; i++ {
		kicked = append(kicked, device.Device{
			DeviceID:  entries[i].deviceID,
			JTI:       entries[i].data.JTI,
			UserUUID:  userUUID,
			CreatedAt: entries[i].data.CreatedAt,
		})
		if err := r.client.HDel(ctx, key, entries[i].deviceID).Err(); err != nil {
			return kicked, fmt.Errorf("redis hdel evict: %w", err)
		}
	}

	return kicked, nil
}
```

- [ ] **Step 2: 创建测试（使用 miniredis）**

```go
package auth

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRedisDeviceStore(t *testing.T) (*RedisDeviceStore, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisDeviceStore(client)

	return store, func() {
		client.Close()
		mr.Close()
	}
}

func TestRedisDeviceStore_AddAndCheck(t *testing.T) {
	store, cleanup := setupRedisDeviceStore(t)
	defer cleanup()
	ctx := context.Background()

	_, err := store.AddDevice(ctx, "user-001", "device-A", "jti-001", 5)
	require.NoError(t, err)

	valid, err := store.CheckDevice(ctx, "user-001", "device-A", "jti-001")
	require.NoError(t, err)
	assert.True(t, valid)
}

func TestRedisDeviceStore_Eviction(t *testing.T) {
	store, cleanup := setupRedisDeviceStore(t)
	defer cleanup()
	ctx := context.Background()

	_, _ = store.AddDevice(ctx, "user-001", "device-A", "jti-A", 2)
	_, _ = store.AddDevice(ctx, "user-001", "device-B", "jti-B", 2)

	kicked, err := store.AddDevice(ctx, "user-001", "device-C", "jti-C", 2)
	require.NoError(t, err)
	assert.Len(t, kicked, 1)
}

func TestRedisDeviceStore_RemoveAll(t *testing.T) {
	store, cleanup := setupRedisDeviceStore(t)
	defer cleanup()
	ctx := context.Background()

	_, _ = store.AddDevice(ctx, "user-001", "device-A", "jti-A", 5)
	_, _ = store.AddDevice(ctx, "user-001", "device-B", "jti-B", 5)

	err := store.RemoveAllDevices(ctx, "user-001")
	require.NoError(t, err)

	list, err := store.ListDevices(ctx, "user-001")
	require.NoError(t, err)
	assert.Empty(t, list)
}
```

- [ ] **Step 3: 添加 miniredis 依赖**

```bash
cd go-middleware && go get github.com/alicebob/miniredis/v2@latest
```

- [ ] **Step 4: 运行测试**

```bash
cd go-middleware && go test ./auth/... -count=1 -v -run TestRedisDevice
```

- [ ] **Step 5: Commit**

```bash
git add go-middleware/auth/redis_device.go go-middleware/auth/redis_device_test.go go-middleware/go.mod go-middleware/go.sum
git commit -m "feat(go-middleware/auth): add RedisDeviceStore"
```

---

### Task 11: 实现 Redis Session Store + Memory Stores

**Files:**
- Create: `go-middleware/auth/redis_session.go`
- Create: `go-middleware/auth/redis_session_test.go`
- Create: `go-middleware/auth/memory_session.go`
- Create: `go-middleware/auth/memory_device.go`
- Create: `go-middleware/auth/memory_test.go`

**Interfaces:**
- Consumes: `go-auth/session.Store`, `go-auth/device.Store`
- Produces: `RedisSessionStore`, `MemorySessionStore`, `MemoryDeviceStore`

- [ ] **Step 1: 创建 go-middleware/auth/redis_session.go**

```go
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/byx-darwin/go-tools/go-auth/session"
	"github.com/redis/go-redis/v9"
)

// RedisSessionStore Redis Session 存储。
type RedisSessionStore struct {
	client redis.UniversalClient
	prefix string
	ttl    time.Duration
}

// NewRedisSessionStore 创建 Redis Session 存储。
func NewRedisSessionStore(client redis.UniversalClient, opts ...Option) *RedisSessionStore {
	cfg := &config{
		prefix:     defaultPrefix,
		sessionTTL: defaultSessionTTL,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return &RedisSessionStore{
		client: client,
		prefix: cfg.prefix,
		ttl:    cfg.sessionTTL,
	}
}

func (r *RedisSessionStore) key(sessionID string) string {
	return fmt.Sprintf("%ssession:%s", r.prefix, sessionID)
}

// Get 获取会话。
func (r *RedisSessionStore) Get(ctx context.Context, sessionID string) (*session.Session, error) {
	raw, err := r.client.Get(ctx, r.key(sessionID)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get session: %w", err)
	}

	var s session.Session
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	if time.Now().After(s.ExpiresAt) {
		return nil, nil
	}

	return &s, nil
}

// Save 保存会话。
func (r *RedisSessionStore) Save(ctx context.Context, s *session.Session) error {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	remaining := time.Until(s.ExpiresAt)
	if remaining <= 0 {
		return r.Delete(ctx, s.ID)
	}

	return r.client.Set(ctx, r.key(s.ID), data, remaining).Err()
}

// Delete 删除会话。
func (r *RedisSessionStore) Delete(ctx context.Context, sessionID string) error {
	return r.client.Del(ctx, r.key(sessionID)).Err()
}

// Exists 检查会话是否存在。
func (r *RedisSessionStore) Exists(ctx context.Context, sessionID string) (bool, error) {
	n, err := r.client.Exists(ctx, r.key(sessionID)).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists: %w", err)
	}
	return n > 0, nil
}
```

- [ ] **Step 2: 创建 go-middleware/auth/memory_session.go**

```go
package auth

import (
	"github.com/byx-darwin/go-tools/go-auth/session"
)

// MemorySessionStore 内存 Session 存储（开发/测试用）。
// 直接包装 go-auth/session.MemoryStore，提供 Options 构造函数以保持一致 API。
type MemorySessionStore struct {
	*session.MemoryStore
}

// NewMemorySessionStore 创建内存 Session 存储。
func NewMemorySessionStore(_ ...Option) *MemorySessionStore {
	return &MemorySessionStore{MemoryStore: session.NewMemoryStore()}
}
```

- [ ] **Step 3: 创建 go-middleware/auth/memory_device.go**

```go
package auth

import (
	"github.com/byx-darwin/go-tools/go-auth/device"
)

// MemoryDeviceStore 内存设备会话存储（开发/测试用）。
type MemoryDeviceStore struct {
	*device.MemoryStore
}

// NewMemoryDeviceStore 创建内存设备存储。
func NewMemoryDeviceStore(_ ...Option) *MemoryDeviceStore {
	return &MemoryDeviceStore{MemoryStore: device.NewMemoryStore()}
}
```

- [ ] **Step 4: 创建测试**

创建 `go-middleware/auth/redis_session_test.go` 和 `go-middleware/auth/memory_test.go`

- [ ] **Step 5: 运行测试**

```bash
cd go-middleware && go test ./auth/... -count=1 -v
```

- [ ] **Step 6: Commit**

```bash
git add go-middleware/auth/
git commit -m "feat(go-middleware/auth): add RedisSessionStore, MemorySessionStore, MemoryDeviceStore"
```

---

### Task 12: 实现 Hertz JWT/Session/Device 中间件

**Files:**
- Create: `go-framework/hertz/middleware/jwt_auth.go`
- Create: `go-framework/hertz/middleware/jwt_auth_test.go`
- Create: `go-framework/hertz/middleware/session_auth.go`
- Create: `go-framework/hertz/middleware/session_auth_test.go`
- Create: `go-framework/hertz/middleware/device_auth.go`
- Create: `go-framework/hertz/middleware/device_auth_test.go`
- Modify: `go-framework/go.mod`（添加 go-auth 依赖）

**Interfaces:**
- Consumes: `go-auth/jwt`, `go-auth/session.Store`, `go-auth/device.Store`, `hertz`
- Produces: `JWTAuth[T]`, `SessionAuth`, `DeviceAuth` 中间件函数

- [ ] **Step 1: 更新 go-framework/go.mod 添加 go-auth 依赖**

添加 require 和 replace（类似 Task 9）

- [ ] **Step 2: 创建 go-framework/hertz/middleware/jwt_auth.go**

```go
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/byx-darwin/go-tools/go-auth/jwt"
	"github.com/cloudwego/hertz/pkg/app"
)

type jwtContextKey string

const jwtClaimsKey jwtContextKey = "jwt_claims"

// JWTAuth JWT 认证中间件。
// 从 Authorization 头解析 Bearer Token，验证签名，将 claims 注入 context。
// 使用方法:
//
//	r.Use(middleware.JWTAuth[MyClaims](jwtSecret))
func JWTAuth[T any](secret []byte, opts ...jwt.Option) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		authHeader := string(c.Request.Header.Peek("Authorization"))
		if authHeader == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		claims, err := jwt.Verify[T](parts[1], secret)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		ctx = context.WithValue(ctx, jwtClaimsKey, claims)
		c.Next(ctx)
	}
}

// GetJWTClaims 从 context 中获取 JWT Claims。
func GetJWTClaims[T any](ctx context.Context) (*T, bool) {
	claims, ok := ctx.Value(jwtClaimsKey).(*T)
	return claims, ok
}
```

- [ ] **Step 3: 创建 go-framework/hertz/middleware/device_auth.go**

```go
package middleware

import (
	"context"
	"net/http"

	"github.com/byx-darwin/go-tools/go-auth/device"
	"github.com/cloudwego/hertz/pkg/app"
)

// DeviceAuth 设备会话检查中间件。
// 需配合 JWTAuth 使用，从 context 中获取 claims，检查设备 jti 是否有效。
func DeviceAuth(store device.Store, maxDevices int) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// 从 context 获取 deviceID 和 jti（由 JWTAuth 中间件注入）
		// 具体实现依赖 Claims 中的 DeviceID 和 JTI 字段
		//
		// 简化版：从请求 Header 获取
		deviceID := string(c.Request.Header.Peek("X-Device-ID"))
		userUUID := string(c.Request.Header.Peek("X-User-UUID"))
		jti := string(c.Request.Header.Peek("X-JTI"))

		if deviceID == "" || userUUID == "" {
			c.Next(ctx)
			return
		}

		valid, err := store.CheckDevice(ctx, userUUID, deviceID, jti)
		if err != nil || !valid {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Next(ctx)
	}
}
```

- [ ] **Step 4: 创建 go-framework/hertz/middleware/session_auth.go**

```go
package middleware

import (
	"context"
	"net/http"

	"github.com/byx-darwin/go-tools/go-auth/session"
	"github.com/cloudwego/hertz/pkg/app"
)

type sessionContextKey string

const sessionCtxKey sessionContextKey = "session"

// SessionAuth Session 认证中间件。
// 从 Cookie 或 Header 解析 Session ID，加载 Session，注入 context。
func SessionAuth(store session.Store) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		sessionID := string(c.Request.Header.Peek("X-Session-ID"))
		if sessionID == "" {
			sessionID = string(c.Request.Header.Cookie("session_id"))
		}
		if sessionID == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		sess, err := store.Get(ctx, sessionID)
		if err != nil || sess == nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		ctx = context.WithValue(ctx, sessionCtxKey, sess)
		c.Next(ctx)
	}
}

// GetSession 从 context 中获取 Session。
func GetSession(ctx context.Context) (*session.Session, bool) {
	sess, ok := ctx.Value(sessionCtxKey).(*session.Session)
	return sess, ok
}
```

- [ ] **Step 5: 创建测试文件**

```go
// go-framework/hertz/middleware/jwt_auth_test.go
package middleware

import (
	"testing"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"

	"github.com/byx-darwin/go-tools/go-auth/jwt"
)

// testClaims 测试用 Claims。
type testClaims struct {
	UserUUID string `json:"user_uuid"`
	jwtlib.RegisteredClaims
}

func TestJWTAuth_SignAndVerify(t *testing.T) {
	secret := []byte("test-secret")
	claims := testClaims{UserUUID: "user-001"}

	token, err := jwt.Sign(claims, secret)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	parsed, err := jwt.Verify[testClaims](token, secret)
	assert.NoError(t, err)
	assert.Equal(t, "user-001", parsed.UserUUID)
}

func TestJWTAuth_InvalidSecret(t *testing.T) {
	secret := []byte("test-secret")
	claims := testClaims{UserUUID: "user-001"}
	token, _ := jwt.Sign(claims, secret)

	_, err := jwt.Verify[testClaims](token, []byte("wrong-secret"))
	assert.Error(t, err)
}
```

- [ ] **Step 6: 运行测试**

```bash
cd go-framework && go test ./hertz/middleware/... -count=1 -v -run "TestJWTAuth|TestSessionAuth|TestDeviceAuth"
```

- [ ] **Step 7: Commit**

```bash
git add go-framework/go.mod go-framework/go.sum go-framework/hertz/middleware/jwt_auth.go go-framework/hertz/middleware/jwt_auth_test.go go-framework/hertz/middleware/session_auth.go go-framework/hertz/middleware/session_auth_test.go go-framework/hertz/middleware/device_auth.go go-framework/hertz/middleware/device_auth_test.go
git commit -m "feat(go-framework): add JWTAuth, SessionAuth, DeviceAuth Hertz middleware"
```

---

### Task 13: 资源清理测试

**Files:**
- Create: `go-auth/device/memory_test.go` 中添加资源清理测试

**Interfaces:**
- 无新增

- [ ] **Step 1: 添加 go-auth/device/memory_test.go 资源清理测试**

```go
func TestMemoryStore_MaxDevicesZero(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// maxDevices <= 0 表示不限制
	for i := range 10 {
		_, err := store.AddDevice(ctx, "user-001", fmt.Sprintf("device-%d", i), fmt.Sprintf("jti-%d", i), 0)
		require.NoError(t, err)
	}

	list, err := store.ListDevices(ctx, "user-001")
	require.NoError(t, err)
	assert.Len(t, list, 10, "maxDevices=0 should not limit")
}
```

- [ ] **Step 2: 运行全部测试验证**

```bash
go test ./go-auth/... ./go-middleware/... ./go-framework/... -count=1
```

- [ ] **Step 3: 运行 lint**

```bash
for m in go-auth go-common go-middleware go-framework; do golangci-lint run --timeout=5m ./$m/...; done
```

- [ ] **Step 4: 运行 build 验证**

```bash
go build ./go-auth/... ./go-common/... ./go-middleware/... ./go-framework/...
```

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(go-auth): add resource cleanup tests and finalize module"
```
