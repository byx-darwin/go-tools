# Store Optional Interface Pattern Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Record an "optional interface" extension convention for `session.Store` / `device.Store`, and land a symmetric `TTLRefresher` example interface in both packages to validate the pattern.

**Architecture:** Two independent, symmetric packages (`go-auth/session`, `go-auth/device`) each get: (1) a godoc addition on their `Store` interface documenting the convention, (2) a new `TTLRefresher` interface with one method, (3) tests proving a mock that implements `TTLRefresher` type-asserts to `ok == true` and a mock that doesn't implements to `ok == false`. No existing `Store` implementation (in go-middleware) is touched.

**Tech Stack:** Go 1.25, `testify/assert` for test assertions (existing pattern in both test files).

**Spec:** `docs/superpowers/specs/2026-09-02-issue-79-store-optional-interface-design.md`

## Global Constraints

- Do NOT modify the `Store` interface method sets in either package (additive-only change).
- Do NOT touch any go-middleware Redis/Memory implementation.
- Do NOT create a new `.claude/rules/` file — convention lives in package doc only.
- All new/modified exported symbols need `// Name ...`-style godoc comments (revive lint rule, `.claude/rules/go.md` §8.3).
- Follow existing mock-based table-test style already present in both `*_test.go` files — do not introduce a new testing library.

---

### Task 1: session package — convention doc + TTLRefresher + tests

**Files:**
- Modify: `go-auth/session/session.go`
- Modify: `go-auth/session/session_test.go`

**Interfaces:**
- Produces: `session.TTLRefresher` interface with method `RefreshTTL(ctx context.Context, sessionID string, ttl time.Duration) error`

- [ ] **Step 1: Write the failing tests**

Append to `go-auth/session/session_test.go` (after the existing `TestStoreInterface` function, same file, same package):

```go
// ── TTLRefresher 可选接口检查 ──

type mockStoreWithTTL struct {
	mockStore
	refreshTTLFn func(ctx context.Context, sessionID string, ttl time.Duration) error
}

func (m *mockStoreWithTTL) RefreshTTL(ctx context.Context, sessionID string, ttl time.Duration) error {
	return m.refreshTTLFn(ctx, sessionID, ttl)
}

// TestTTLRefresherOptionalInterface 验证 TTLRefresher 作为可选接口的类型断言行为。
func TestTTLRefresherOptionalInterface(t *testing.T) {
	t.Run("store implementing TTLRefresher asserts ok", func(t *testing.T) {
		var calledID string
		var calledTTL time.Duration
		var store Store = &mockStoreWithTTL{
			refreshTTLFn: func(_ context.Context, sessionID string, ttl time.Duration) error {
				calledID = sessionID
				calledTTL = ttl
				return nil
			},
		}

		refresher, ok := store.(TTLRefresher)
		assert.True(t, ok)

		err := refresher.RefreshTTL(context.Background(), "sid-1", 5*time.Minute)
		assert.NoError(t, err)
		assert.Equal(t, "sid-1", calledID)
		assert.Equal(t, 5*time.Minute, calledTTL)
	})

	t.Run("store implementing TTLRefresher propagates error", func(t *testing.T) {
		var store Store = &mockStoreWithTTL{
			refreshTTLFn: func(_ context.Context, _ string, _ time.Duration) error {
				return errors.New("refresh error")
			},
		}

		refresher, ok := store.(TTLRefresher)
		assert.True(t, ok)

		err := refresher.RefreshTTL(context.Background(), "sid-1", time.Minute)
		assert.Error(t, err)
	})

	t.Run("store not implementing TTLRefresher asserts not ok", func(t *testing.T) {
		var store Store = &mockStore{}

		_, ok := store.(TTLRefresher)
		assert.False(t, ok)
	})
}

var (
	_ Store        = (*mockStoreWithTTL)(nil)
	_ TTLRefresher = (*mockStoreWithTTL)(nil)
)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./go-auth/session/... -run TestTTLRefresherOptionalInterface -v`
Expected: FAIL — compile error `undefined: TTLRefresher`

- [ ] **Step 3: Add the convention doc + TTLRefresher interface**

In `go-auth/session/session.go`, insert the following godoc block immediately above the existing `// Store Session 存储接口。` comment (i.e., between the `Session` struct and the `Store` interface), and add the `TTLRefresher` type after the `Store` interface's closing brace:

```go
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
```

Note: `session.go` already imports `context` and `time` — no import changes needed.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./go-auth/session/... -v`
Expected: PASS (all tests, including pre-existing ones)

- [ ] **Step 5: Lint check**

Run: `golangci-lint run --timeout=5m ./go-auth/...`
Expected: no new findings

- [ ] **Step 6: Commit**

```bash
git add go-auth/session/session.go go-auth/session/session_test.go
git commit -m "docs(go-auth): document Store optional-interface extension convention, add TTLRefresher to session"
```

---

### Task 2: device package — convention doc + TTLRefresher + tests

**Files:**
- Modify: `go-auth/device/store.go`
- Modify: `go-auth/device/store_test.go`

**Interfaces:**
- Produces: `device.TTLRefresher` interface with method `RefreshTTL(ctx context.Context, userUUID, deviceID string, ttl time.Duration) error`

- [ ] **Step 1: Write the failing tests**

Append to `go-auth/device/store_test.go` (after the existing `TestStoreInterface` function, same file, same package):

```go
// ── TTLRefresher 可选接口检查 ──

type mockStoreWithTTL struct {
	mockStore
	refreshTTLFn func(ctx context.Context, userUUID, deviceID string, ttl time.Duration) error
}

func (m *mockStoreWithTTL) RefreshTTL(ctx context.Context, userUUID, deviceID string, ttl time.Duration) error {
	return m.refreshTTLFn(ctx, userUUID, deviceID, ttl)
}

// TestTTLRefresherOptionalInterface 验证 TTLRefresher 作为可选接口的类型断言行为。
func TestTTLRefresherOptionalInterface(t *testing.T) {
	t.Run("store implementing TTLRefresher asserts ok", func(t *testing.T) {
		var calledUser, calledDevice string
		var calledTTL time.Duration
		var store Store = &mockStoreWithTTL{
			refreshTTLFn: func(_ context.Context, userUUID, deviceID string, ttl time.Duration) error {
				calledUser = userUUID
				calledDevice = deviceID
				calledTTL = ttl
				return nil
			},
		}

		refresher, ok := store.(TTLRefresher)
		assert.True(t, ok)

		err := refresher.RefreshTTL(context.Background(), "user-1", "device-1", 5*time.Minute)
		assert.NoError(t, err)
		assert.Equal(t, "user-1", calledUser)
		assert.Equal(t, "device-1", calledDevice)
		assert.Equal(t, 5*time.Minute, calledTTL)
	})

	t.Run("store implementing TTLRefresher propagates error", func(t *testing.T) {
		var store Store = &mockStoreWithTTL{
			refreshTTLFn: func(_ context.Context, _, _ string, _ time.Duration) error {
				return errors.New("refresh error")
			},
		}

		refresher, ok := store.(TTLRefresher)
		assert.True(t, ok)

		err := refresher.RefreshTTL(context.Background(), "user-1", "device-1", time.Minute)
		assert.Error(t, err)
	})

	t.Run("store not implementing TTLRefresher asserts not ok", func(t *testing.T) {
		var store Store = &mockStore{}

		_, ok := store.(TTLRefresher)
		assert.False(t, ok)
	})
}

var (
	_ Store        = (*mockStoreWithTTL)(nil)
	_ TTLRefresher = (*mockStoreWithTTL)(nil)
)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./go-auth/device/... -run TestTTLRefresherOptionalInterface -v`
Expected: FAIL — compile error `undefined: TTLRefresher`

- [ ] **Step 3: Add the convention doc + TTLRefresher interface**

In `go-auth/device/store.go`, insert the following godoc block immediately above the existing `// Store 设备会话存储接口。` comment, and add the `TTLRefresher` type after the `Store` interface's closing brace:

```go
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
```

Note: `store.go` currently imports only `"context"` — add `"time"` to the import block (standard library group, per goimports grouping rule).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./go-auth/device/... -v`
Expected: PASS (all tests, including pre-existing ones)

- [ ] **Step 5: Lint check**

Run: `golangci-lint run --timeout=5m ./go-auth/...`
Expected: no new findings

- [ ] **Step 6: Commit**

```bash
git add go-auth/device/store.go go-auth/device/store_test.go
git commit -m "docs(go-auth): document Store optional-interface extension convention, add TTLRefresher to device"
```

---

### Task 3: Full workspace validation

**Files:** none (validation only)

- [ ] **Step 1: Build the whole workspace**

Run: `go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...`
Expected: success, no errors (confirms go-middleware's untouched Redis/Memory implementations still satisfy `Store` — they are not required to satisfy `TTLRefresher`)

- [ ] **Step 2: Vet the whole workspace**

Run: `go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...`
Expected: no issues

- [ ] **Step 3: Run full test suite**

Run: `go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1`
Expected: all PASS
