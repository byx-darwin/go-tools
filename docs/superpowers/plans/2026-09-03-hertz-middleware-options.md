# CORS/SessionAuth 中间件 Options 化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `go-framework/hertz/middleware` 包内 `Cors()` 和 `SessionAuth()` 两个硬编码参数的中间件改造为 Functional Options 模式，默认行为完全向后兼容。

**Architecture:** 各自新增一个私有 config struct + `<Middleware>Option` 函数类型 + `WithXxx` 选项函数，构造函数签名追加 `opts ...XxxOption` 可变参数；不改变现有导出签名的必填参数，不引入新文件。

**Tech Stack:** Go 1.25, Hertz (`github.com/cloudwego/hertz`), `testify/assert`, `hertz/pkg/common/ut`（测试用请求模拟）。

**Spec:** `docs/superpowers/specs/2026-09-03-hertz-middleware-options-design.md`

## Global Constraints

- 遵循 `.claude/rules/options-pattern.md`：`Option` 类型命名为 `<Middleware>Option`（因包级 `Option` 已被 `auth.go` 占用）；每个 `WithXxx` 只设置一个字段；对空值/非法输入做防御，不覆盖默认值；构造函数先填默认值再应用 opts；必须有 godoc 注释。
- 遵循 `.claude/rules/go.md` § 8：所有导出符号必须有 `// Name ...` 格式 godoc 注释；`gofmt`/`goimports`（标准库/第三方/本项目三组）；`errcheck`；八进制字面量用 `0o` 前缀（本次改动不涉及）；`any` 而非 `interface{}`。
- `Cors()` / `SessionAuth(store)` 零参数/原有参数调用的输出/行为必须与改造前逐字节一致（现有测试若存在必须继续通过）。
- 不修改 `auth.go`/`jwt_auth.go`/`accesslog.go`/`device_auth.go`/`server.go`（Recovery）。
- 不新建文件；`CorsOption`/`WithAllowOrigins` 等类型和函数直接加在 `cors.go`；`SessionAuthOption` 等加在 `session_auth.go`。

---

### Task 1: `SessionAuthOption` — Header/Cookie 名称可配置

**Files:**
- Modify: `go-framework/hertz/middleware/session_auth.go`
- Test: `go-framework/hertz/middleware/session_auth_test.go`

**Interfaces:**
- Consumes: 无跨任务依赖，仅用包内已有 `session.Store` 接口、`extractSessionID(c *app.RequestContext) string`（本任务会改造其签名）。
- Produces:
  - `type SessionAuthOption func(*sessionAuthConfig)`
  - `func WithSessionHeader(name string) SessionAuthOption`
  - `func WithSessionCookie(name string) SessionAuthOption`
  - `func SessionAuth(store session.Store, opts ...SessionAuthOption) app.HandlerFunc`（原签名 `SessionAuth(store session.Store) app.HandlerFunc` 追加可变参数，调用方零改动兼容）
  - `extractSessionID(c *app.RequestContext, headerName, cookieName string) string`（内部函数签名变化，仅包内使用，Task 2 不依赖它）

- [ ] **Step 1: 写失败测试 — 自定义 Header 名称生效**

在 `go-framework/hertz/middleware/session_auth_test.go` 追加：

```go
func TestSessionAuth_CustomHeaderName(t *testing.T) {
	store := newMockSessionStore()
	s := &session.Session{ID: "sess-custom", UserUUID: "user-custom", ExpiresAt: time.Now().Add(time.Hour)}
	_ = store.Save(context.Background(), s)

	engine := newSessionTestEngine()
	engine.Use(SessionAuth(store, WithSessionHeader("X-My-Session")))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		got, ok := GetSession(c)
		assert.True(t, ok)
		c.JSON(http.StatusOK, map[string]string{"user": got.UserUUID})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "X-My-Session", Value: "sess-custom"})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
	assert.Contains(t, string(res.Body()), "user-custom")
}

func TestSessionAuth_CustomHeaderName_DefaultHeaderIgnored(t *testing.T) {
	store := newMockSessionStore()
	s := &session.Session{ID: "sess-custom2", UserUUID: "user-custom2", ExpiresAt: time.Now().Add(time.Hour)}
	_ = store.Save(context.Background(), s)

	engine := newSessionTestEngine()
	engine.Use(SessionAuth(store, WithSessionHeader("X-My-Session")))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	// 默认头 X-Session-Id 不再生效，应视为未提供 session id。
	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "X-Session-Id", Value: "sess-custom2"})
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode())
}

func TestSessionAuth_CustomCookieName(t *testing.T) {
	store := newMockSessionStore()
	s := &session.Session{ID: "sess-ck", UserUUID: "user-ck", ExpiresAt: time.Now().Add(time.Hour)}
	_ = store.Save(context.Background(), s)

	engine := newSessionTestEngine()
	engine.Use(SessionAuth(store, WithSessionCookie("my_sid")))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		got, ok := GetSession(c)
		assert.True(t, ok)
		c.JSON(http.StatusOK, map[string]string{"user": got.UserUUID})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Cookie", Value: "my_sid=sess-ck"})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
	assert.Contains(t, string(res.Body()), "user-ck")
}

func TestSessionAuth_EmptyOptionValuesIgnored(t *testing.T) {
	store := newMockSessionStore()
	s := &session.Session{ID: "sess-def", UserUUID: "user-def", ExpiresAt: time.Now().Add(time.Hour)}
	_ = store.Save(context.Background(), s)

	engine := newSessionTestEngine()
	// 空字符串 Option 不应覆盖默认值。
	engine.Use(SessionAuth(store, WithSessionHeader(""), WithSessionCookie("")))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		got, ok := GetSession(c)
		assert.True(t, ok)
		c.JSON(http.StatusOK, map[string]string{"user": got.UserUUID})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "X-Session-Id", Value: "sess-def"})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
	assert.Contains(t, string(res.Body()), "user-def")
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-framework/hertz/middleware/... -run TestSessionAuth_Custom -v`
Expected: 编译失败（`WithSessionHeader`/`WithSessionCookie` undefined）。

- [ ] **Step 3: 最小实现**

修改 `go-framework/hertz/middleware/session_auth.go`：

```go
package middleware

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/byx-darwin/go-tools/go-auth/session"
)

const (
	// headerSessionID Session ID 请求头默认名称。
	headerSessionID = "X-Session-Id"
	// cookieSessionID Session ID Cookie 默认名称。
	cookieSessionID = "session_id"
)

// sessionAuthConfig 存储 SessionAuth 配置选项。
type sessionAuthConfig struct {
	headerName string
	cookieName string
}

// SessionAuthOption 定义 SessionAuth 配置选项函数。
type SessionAuthOption func(*sessionAuthConfig)

// WithSessionHeader 设置 Session ID 请求头名称。空字符串忽略，保持默认 X-Session-Id。
func WithSessionHeader(name string) SessionAuthOption {
	return func(c *sessionAuthConfig) {
		if name != "" {
			c.headerName = name
		}
	}
}

// WithSessionCookie 设置 Session ID Cookie 名称。空字符串忽略，保持默认 session_id。
func WithSessionCookie(name string) SessionAuthOption {
	return func(c *sessionAuthConfig) {
		if name != "" {
			c.cookieName = name
		}
	}
}

// SessionAuth 返回 Session 认证中间件。
// 优先从 Header（默认 X-Session-Id，可用 WithSessionHeader 定制）解析 Session ID，
// 其次从 Cookie（默认 session_id，可用 WithSessionCookie 定制）解析。
// 验证通过后将 Session 注入 RequestContext。
//
// 使用方式：
//
//	engine.Use(middleware.SessionAuth(sessionStore))
//	s, ok := middleware.GetSession(c)
//
//	// 自定义 Header/Cookie 名称：
//	engine.Use(middleware.SessionAuth(sessionStore,
//	    middleware.WithSessionHeader("X-My-Session"),
//	    middleware.WithSessionCookie("my_sid")))
func SessionAuth(store session.Store, opts ...SessionAuthOption) app.HandlerFunc {
	cfg := sessionAuthConfig{headerName: headerSessionID, cookieName: cookieSessionID}
	for _, opt := range opts {
		opt(&cfg)
	}

	return func(ctx context.Context, c *app.RequestContext) {
		sessionID := extractSessionID(c, cfg.headerName, cfg.cookieName)
		if sessionID == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		s, err := store.Get(ctx, sessionID)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if s == nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		SetSession(c, s)
		c.Next(ctx)
	}
}

// extractSessionID 从请求中提取 Session ID。
// 优先级：headerName 请求头 > cookieName Cookie。
func extractSessionID(c *app.RequestContext, headerName, cookieName string) string {
	// 优先从 Header 获取。
	if id := string(c.Request.Header.Peek(headerName)); id != "" {
		return id
	}
	// 其次从 Cookie 获取。
	if id := string(c.Cookie(cookieName)); id != "" {
		return id
	}
	return ""
}
```

- [ ] **Step 4: 运行全部 session_auth 测试确认通过**

Run: `go test ./go-framework/hertz/middleware/... -run TestSessionAuth -v`
Expected: 全部 PASS（含原有 `TestSessionAuth_SuccessFromHeader` 等 5 个用例 + 本任务新增 4 个用例）。

- [ ] **Step 5: Commit**

```bash
git add go-framework/hertz/middleware/session_auth.go go-framework/hertz/middleware/session_auth_test.go
git commit -m "feat(go-framework/hertz): SessionAuth 支持自定义 Header/Cookie 名称"
```

---

### Task 2: `CorsOption` — Origin/Header/Method/ExposeHeaders/Credentials 可配置

**Files:**
- Modify: `go-framework/hertz/middleware/cors.go`
- Test: Create `go-framework/hertz/middleware/cors_test.go`（当前不存在该测试文件）

**Interfaces:**
- Consumes: 无跨任务依赖（与 Task 1 互不影响，`Cors()` 与 `SessionAuth()` 无共享状态）。
- Produces:
  - `type CorsOption func(*corsConfig)`
  - `func WithAllowOrigins(origins []string) CorsOption`
  - `func WithAllowHeaders(headers []string) CorsOption`
  - `func WithAllowMethods(methods []string) CorsOption`
  - `func WithExposeHeaders(headers []string) CorsOption`
  - `func WithAllowCredentials(allow bool) CorsOption`
  - `func Cors(opts ...CorsOption) app.HandlerFunc`（原签名 `Cors() app.HandlerFunc` 追加可变参数，`Cors()` 零参数调用兼容）

- [ ] **Step 1: 写失败测试 — 默认行为与自定义 Origin 白名单**

创建 `go-framework/hertz/middleware/cors_test.go`：

```go
package middleware

import (
	"context"
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/stretchr/testify/assert"
)

func newCorsTestEngine() *route.Engine {
	opt := config.NewOptions([]config.Option{})
	return route.NewEngine(opt)
}

func TestCors_DefaultBehavior_Unchanged(t *testing.T) {
	engine := newCorsTestEngine()
	engine.Use(Cors())
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Origin", Value: "https://example.com"})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
	assert.Equal(t, "*", string(res.Header.Peek("Access-Control-Allow-Origin")))
	assert.Equal(t, "Content-Type,X-Authorization, X-Signature", string(res.Header.Peek("Access-Control-Allow-Headers")))
	assert.Equal(t, "POST, GET, OPTIONS,DELETE,PUT", string(res.Header.Peek("Access-Control-Allow-Methods")))
	assert.Equal(t, "Content-Length, Access-Control-Allow-Origin,New-Token,New-Expires-At,Access-Control-Allow-Headers, Content-Type", string(res.Header.Peek("Access-Control-Expose-Headers")))
	assert.Equal(t, "true", string(res.Header.Peek("Access-Control-Allow-Credentials")))
}

func TestCors_DefaultBehavior_OptionsRequest_NoContent(t *testing.T) {
	engine := newCorsTestEngine()
	engine.Use(Cors())
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "OPTIONS", "/test", &ut.Body{Body: nil})
	res := w.Result()
	assert.Equal(t, http.StatusNoContent, res.StatusCode())
}

func TestCors_AllowOrigins_WhitelistMatch(t *testing.T) {
	engine := newCorsTestEngine()
	engine.Use(Cors(WithAllowOrigins([]string{"https://a.example.com", "https://b.example.com"})))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Origin", Value: "https://a.example.com"})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
	assert.Equal(t, "https://a.example.com", string(res.Header.Peek("Access-Control-Allow-Origin")))
}

func TestCors_AllowOrigins_WhitelistMiss_HeaderOmitted(t *testing.T) {
	engine := newCorsTestEngine()
	engine.Use(Cors(WithAllowOrigins([]string{"https://a.example.com"})))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Origin", Value: "https://evil.example.com"})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
	assert.Equal(t, "", string(res.Header.Peek("Access-Control-Allow-Origin")))
}

func TestCors_CustomHeadersMethodsExposeCredentials(t *testing.T) {
	engine := newCorsTestEngine()
	engine.Use(Cors(
		WithAllowHeaders([]string{"Content-Type", "X-Custom"}),
		WithAllowMethods([]string{"GET", "POST"}),
		WithExposeHeaders([]string{"X-Total-Count"}),
		WithAllowCredentials(false),
	))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Origin", Value: "https://example.com"})
	res := w.Result()
	assert.Equal(t, "Content-Type,X-Custom", string(res.Header.Peek("Access-Control-Allow-Headers")))
	assert.Equal(t, "GET,POST", string(res.Header.Peek("Access-Control-Allow-Methods")))
	assert.Equal(t, "X-Total-Count", string(res.Header.Peek("Access-Control-Expose-Headers")))
	assert.Equal(t, "false", string(res.Header.Peek("Access-Control-Allow-Credentials")))
}

func TestCors_EmptyOptionValuesIgnored(t *testing.T) {
	engine := newCorsTestEngine()
	// 空 slice 不应覆盖默认值。
	engine.Use(Cors(WithAllowOrigins(nil), WithAllowHeaders(nil), WithAllowMethods(nil), WithExposeHeaders(nil)))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Origin", Value: "https://example.com"})
	res := w.Result()
	assert.Equal(t, "*", string(res.Header.Peek("Access-Control-Allow-Origin")))
	assert.Equal(t, "Content-Type,X-Authorization, X-Signature", string(res.Header.Peek("Access-Control-Allow-Headers")))
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-framework/hertz/middleware/... -run TestCors -v`
Expected: 编译失败（`CorsOption`/`WithAllowOrigins` 等 undefined）。

- [ ] **Step 3: 最小实现**

修改 `go-framework/hertz/middleware/cors.go`：

```go
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
)

// corsConfig 存储 Cors 配置选项。
type corsConfig struct {
	allowOrigins     []string
	allowHeaders     []string
	allowMethods     []string
	exposeHeaders    []string
	allowCredentials bool
}

// CorsOption 定义 Cors 配置选项函数。
type CorsOption func(*corsConfig)

// WithAllowOrigins 设置允许跨域的 Origin 白名单。空 slice 忽略，保持默认 ["*"]。
// 显式设置为非 ["*"] 值后，仅当请求 Origin 命中白名单时才回显该 Origin；
// 未命中时不设置 Access-Control-Allow-Origin 响应头。
func WithAllowOrigins(origins []string) CorsOption {
	return func(c *corsConfig) {
		if len(origins) > 0 {
			c.allowOrigins = origins
		}
	}
}

// WithAllowHeaders 设置 Access-Control-Allow-Headers 列表。空 slice 忽略。
func WithAllowHeaders(headers []string) CorsOption {
	return func(c *corsConfig) {
		if len(headers) > 0 {
			c.allowHeaders = headers
		}
	}
}

// WithAllowMethods 设置 Access-Control-Allow-Methods 列表。空 slice 忽略。
func WithAllowMethods(methods []string) CorsOption {
	return func(c *corsConfig) {
		if len(methods) > 0 {
			c.allowMethods = methods
		}
	}
}

// WithExposeHeaders 设置 Access-Control-Expose-Headers 列表。空 slice 忽略。
func WithExposeHeaders(headers []string) CorsOption {
	return func(c *corsConfig) {
		if len(headers) > 0 {
			c.exposeHeaders = headers
		}
	}
}

// WithAllowCredentials 设置 Access-Control-Allow-Credentials。
func WithAllowCredentials(allow bool) CorsOption {
	return func(c *corsConfig) {
		c.allowCredentials = allow
	}
}

// defaultAllowOrigins Cors 默认 Origin 白名单（通配符，保持历史行为）。
var defaultAllowOrigins = []string{"*"}

// defaultAllowHeaders Cors 默认允许的请求头列表。
var defaultAllowHeaders = []string{"Content-Type,X-Authorization, X-Signature"}

// defaultAllowMethods Cors 默认允许的请求方法列表。
var defaultAllowMethods = []string{"POST, GET, OPTIONS,DELETE,PUT"}

// defaultExposeHeaders Cors 默认暴露的响应头列表。
var defaultExposeHeaders = []string{"Content-Length, Access-Control-Allow-Origin,New-Token,New-Expires-At,Access-Control-Allow-Headers, Content-Type"}

// Cors 返回 CORS 中间件，允许跨域请求。
// 默认行为与历史版本完全一致：Origin 为 "*"，Header/Method/ExposeHeaders 为固定列表，
// Access-Control-Allow-Credentials 为 true。可通过 Option 定制：
//
//	engine.Use(middleware.Cors())
//
//	// 限制 Origin 白名单：
//	engine.Use(middleware.Cors(middleware.WithAllowOrigins([]string{"https://app.example.com"})))
func Cors(opts ...CorsOption) app.HandlerFunc {
	cfg := corsConfig{
		allowOrigins:     defaultAllowOrigins,
		allowHeaders:     defaultAllowHeaders,
		allowMethods:     defaultAllowMethods,
		exposeHeaders:    defaultExposeHeaders,
		allowCredentials: true,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	usesWildcard := len(cfg.allowOrigins) == 1 && cfg.allowOrigins[0] == "*"
	allowHeaders := strings.Join(cfg.allowHeaders, ",")
	allowMethods := strings.Join(cfg.allowMethods, ",")
	exposeHeaders := strings.Join(cfg.exposeHeaders, ",")

	return func(c context.Context, ctx *app.RequestContext) {
		if usesWildcard {
			ctx.Header("Access-Control-Allow-Origin", "*")
		} else if origin := string(ctx.Request.Header.Peek("Origin")); origin != "" && originAllowed(origin, cfg.allowOrigins) {
			ctx.Header("Access-Control-Allow-Origin", origin)
		}
		ctx.Header("Access-Control-Allow-Headers", allowHeaders)
		ctx.Header("Access-Control-Allow-Methods", allowMethods)
		ctx.Header("Access-Control-Expose-Headers", exposeHeaders)
		if cfg.allowCredentials {
			ctx.Header("Access-Control-Allow-Credentials", "true")
		} else {
			ctx.Header("Access-Control-Allow-Credentials", "false")
		}

		if string(ctx.Request.Method()) == "OPTIONS" {
			ctx.AbortWithStatus(http.StatusNoContent)
		}
		ctx.Next(c)
	}
}

// originAllowed 判断 origin 是否命中白名单。
func originAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == origin {
			return true
		}
	}
	return false
}
```

注意：默认值需与改造前逐字节一致 —— 原代码里 Header/Method/ExposeHeaders 常量本身就是**单个完整字符串**（内部含逗号+空格，不是真正的 slice），因此 `defaultAllowHeaders` 等默认 slice 只放一个元素（该完整字符串），`strings.Join(cfg.allowHeaders, ",")` 对单元素 slice 直接返回该元素本身，与原始 `ctx.Header(...)` 参数逐字节相同；用户如果传入多元素 slice（如 Task 测试里的 `[]string{"Content-Type", "X-Custom"}`），才会真正体现"用逗号拼接多个值"的语义。

- [ ] **Step 4: 运行全部 cors 测试确认通过**

Run: `go test ./go-framework/hertz/middleware/... -run TestCors -v`
Expected: 全部 PASS（7 个用例）。

- [ ] **Step 5: 运行整包测试确认无回归**

Run: `go test ./go-framework/hertz/middleware/... -count=1 -v`
Expected: 全部 PASS，包括 Task 1 新增用例、原有 `auth_test.go`/`jwt_auth_test.go`/`device_auth_test.go`/`accesslog_test.go`/`context_test.go` 用例。

- [ ] **Step 6: Commit**

```bash
git add go-framework/hertz/middleware/cors.go go-framework/hertz/middleware/cors_test.go
git commit -m "feat(go-framework/hertz): Cors 支持 Origin 白名单等可配置项"
```

---

### Task 3: 静态检查与整包验证

**Files:** 无新增/修改文件，仅运行检查命令。

**Interfaces:**
- Consumes: Task 1、Task 2 产出的全部代码变更。
- Produces: 验证结果（PASS/FAIL），供 Gate 2→3 / `gf-quality` 复核。

- [ ] **Step 1: gofmt 检查**

Run: `gofmt -l go-framework/hertz/middleware/cors.go go-framework/hertz/middleware/session_auth.go go-framework/hertz/middleware/cors_test.go go-framework/hertz/middleware/session_auth_test.go`
Expected: 无输出（已格式化）。

- [ ] **Step 2: go vet**

Run: `go vet ./go-framework/...`
Expected: 无输出。

- [ ] **Step 3: golangci-lint（仅本 module）**

Run: `golangci-lint run --timeout=5m ./go-framework/...`
Expected: 无新增 lint 问题（重点关注 `revive` 导出符号注释、`unconvert`、`gocritic`）。

- [ ] **Step 4: 完整 module 测试**

Run: `go test ./go-framework/... -count=1`
Expected: 全部 PASS。

- [ ] **Step 5: 无需 commit（本任务不产生文件变更）**

---

## Self-Review Notes（供实现前复核）

- Spec 覆盖：`CorsOption` 五个选项 ✅ Task 2；`SessionAuthOption` 两个选项 ✅ Task 1；默认行为不变的回归断言 ✅ 两个任务的 `Default`/`Unchanged` 用例；排查结论已记录在设计文档，无需额外任务。
- 类型一致性：`Cors(opts ...CorsOption)`、`SessionAuth(store session.Store, opts ...SessionAuthOption)` 与 Task 1/2 内部实现签名一致；`extractSessionID` 签名变化仅限包内私有函数，无外部调用方。
- 无占位符：所有 Step 均含可直接使用的完整代码。
