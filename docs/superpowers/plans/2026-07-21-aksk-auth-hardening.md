# AK/SK 中间件安全加固 Implementation Plan（#21 / #22 / #23）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 `go-framework/hertz/middleware/auth.go` 的三处 AK/SK 安全缺陷：调试模式回显有效签名（#21）、签名不覆盖 method/query/body（#22）、不强制时间戳新鲜度窗口（#23）。

**Architecture:** 重写规范签名格式为换行分隔串 `ak\nmethod\nrequestURI\ntimestamp\nsha256hex(body)` 并 HMAC-SHA256；用 `hmac.Equal` 常量时间比较；用 Functional Options 提供可配置时间戳窗口并在 `GetSk` 前强制校验；`parseAuthorization` 不再吞 ParseInt 错误。全部改动集中在 `auth.go` 与 `auth_test.go` 两个文件。

**Tech Stack:** Go 1.25 · Hertz v0.10.0（`route.Engine` + `pkg/common/ut` 测试）· testify（assert/require）· go-common/error（oops 风格）。

## Global Constraints

- 遵循 `.claude/rules/go.md`：gofmt/goimports 干净、导出符号必须有 `// Name ...` godoc、错误必须处理、`0o` 八进制、`any` 替代 `interface{}`。
- 遵循 `.claude/rules/options-pattern.md`：新增配置走 `Option` + `WithXxx`，默认值集中定义，对无效输入防御。
- **不越界**：不改 `example/`（未注册 AKSK Auth）；不改 `go-common/auth/ak.go` 的 `RefreshSK`/MD5（属 #24 组 C）。
- 验证命令：`go test ./go-framework/... -count=1`；lint：`golangci-lint run --timeout=5m ./go-framework/...`（需 v2）。
- 工作分支：`feat/21-aksk-auth-hardening`（worktree 内）。
- 提交信息遵循 conventional commits（`fix(go-framework): ...`），引用 #21/#22/#23。

---

## File Structure

| 文件 | 责任 | 变更 |
|------|------|------|
| `go-framework/hertz/middleware/auth.go` | AK/SK 验签中间件 + 规范签名算法 + Options | 重写 |
| `go-framework/hertz/middleware/auth_test.go` | 上述全部行为的单元 + 中间件级测试 | 重写 |

无新增文件。`signRequest` 改为 6 参（`ak, sk, method, requestURI string, t int64, body []byte`），`Auth` 增加 variadic `opts ...Option`（源码级向后兼容）。

---

## Task 1: 规范签名算法（#22 核心）— `signRequest` 新格式

**Files:**
- Modify: `go-framework/hertz/middleware/auth.go`（`signRequest`，约 `:106-112`）
- Test: `go-framework/hertz/middleware/auth_test.go`

**Interfaces:**
- Produces: `func signRequest(ak, sk, method, requestURI string, t int64, body []byte) string` —— 返回 `hex(HMAC-SHA256(sk, ak+"\n"+method+"\n"+requestURI+"\n"+t+"\n"+sha256hex(body)))`。后续 Task 2/3 的中间件与测试均依赖此签名。

- [ ] **Step 1: 写失败测试（替换 auth_test.go 中全部旧 signRequest 测试）**

将 `auth_test.go` 内容替换为：

```go
package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignRequest_Deterministic(t *testing.T) {
	body := []byte(`{"a":1}`)
	s1 := signRequest("ak", "sk", "POST", "/p?x=1", 1700000000, body)
	s2 := signRequest("ak", "sk", "POST", "/p?x=1", 1700000000, body)
	assert.Equal(t, s1, s2)
	assert.Len(t, s1, 64, "HMAC-SHA256 hex should be 64 chars")
}

func TestSignRequest_BindsMethodQueryBody(t *testing.T) {
	base := signRequest("ak", "sk", "GET", "/t?amount=1", 1700000000, nil)

	// 改 method
	assert.NotEqual(t, base, signRequest("ak", "sk", "POST", "/t?amount=1", 1700000000, nil),
		"different method must change signature")
	// 改 query
	assert.NotEqual(t, base, signRequest("ak", "sk", "GET", "/t?amount=9999", 1700000000, nil),
		"different query must change signature")
	// 改 body
	assert.NotEqual(t, base, signRequest("ak", "sk", "GET", "/t?amount=1", 1700000000, []byte("x")),
		"different body must change signature")
	// 改 ak / sk / timestamp
	assert.NotEqual(t, base, signRequest("other", "sk", "GET", "/t?amount=1", 1700000000, nil))
	assert.NotEqual(t, base, signRequest("ak", "sk2", "GET", "/t?amount=1", 1700000000, nil))
	assert.NotEqual(t, base, signRequest("ak", "sk", "GET", "/t?amount=1", 1700000001, nil))
}

func TestSignRequest_EmptyBodyUsesEmptySHA256(t *testing.T) {
	empty := sha256.Sum256(nil)
	msg := fmt.Sprintf("%s\n%s\n%s\n%d\n%s", "ak", "GET", "/p", int64(1), hex.EncodeToString(empty[:]))
	// 手工构造期望值，验证规范化消息格式
	_ = msg
	s1 := signRequest("ak", "sk", "GET", "/p", 1, nil)
	s2 := signRequest("ak", "sk", "GET", "/p", 1, []byte{})
	assert.Equal(t, s1, s2, "nil body and empty body must hash identically")
}

func TestSignRequest_HexFormat(t *testing.T) {
	sign := signRequest("ak", "sk", "GET", "/p", 12345, nil)
	for _, ch := range sign {
		assert.True(t, (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f'),
			"sign must be lowercase hex, got '%c'", ch)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd go-framework && go test ./hertz/middleware/ -run TestSignRequest -count=1`
Expected: **编译失败**（`signRequest` 仍是 4 参，调用处 6 参不匹配）。

- [ ] **Step 3: 实现新 `signRequest`**

替换 `auth.go` 中 `signRequest`（并删除不再使用的 `fmt` import，新增 `strings` 已在）：

```go
// signRequest 计算请求签名。
//
// 规范签名格式（client 与 server 必须一致）：
//
//	stringToSign = ak + "\n" + method + "\n" + requestURI + "\n" + timestamp + "\n" + sha256hex(body)
//	signature    = hex( HMAC-SHA256( key = sk, msg = stringToSign ) )
//
// method 为大写 HTTP 方法；requestURI 为 origin-form 请求目标（path?query）；
// timestamp 为十进制秒级时间戳；sha256hex(body) 为原始 body 的 SHA-256 小写十六进制
// （无 body 时为空输入的 SHA-256）。
func signRequest(ak, sk, method, requestURI string, t int64, body []byte) string {
	bodyHash := sha256.Sum256(body)
	msg := strings.Join([]string{
		ak,
		method,
		requestURI,
		strconv.FormatInt(t, 10),
		hex.EncodeToString(bodyHash[:]),
	}, "\n")
	h := hmac.New(sha256.New, []byte(sk))
	h.Write([]byte(msg))
	return hex.EncodeToString(h.Sum(nil))
}
```

注意：此时 `auth.go` 中 `Auth` 里对 `signRequest(ak, sk, string(c.Request.Path()), t)` 的调用会编译失败——Task 2 会一并修正。为让 Task 1 可独立编译验证，**本步同时**把 `Auth` 内那一行临时改为新签名调用（与 Task 2 最终一致）：

```go
		expected := signRequest(ak, sk, string(c.Request.Method()),
			string(c.Request.RequestURI()), t, c.Request.Body())
```

并删除 `auth.go` import 中的 `"fmt"`（若此后无其它使用）。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd go-framework && go test ./hertz/middleware/ -run TestSignRequest -count=1`
Expected: **PASS**（4 个测试）。

- [ ] **Step 5: 提交**

```bash
git add go-framework/hertz/middleware/auth.go go-framework/hertz/middleware/auth_test.go
git commit -m "fix(go-framework): AK/SK signature covers method, query and body (#22)"
```

---

## Task 2: 消除凭据预言机 + 常量时间比较（#21）

**Files:**
- Modify: `go-framework/hertz/middleware/auth.go`（`Auth`，约 `:29-59`）
- Test: `go-framework/hertz/middleware/auth_test.go`

**Interfaces:**
- Consumes: `signRequest(ak, sk, method, requestURI string, t int64, body []byte) string`（Task 1）
- Produces: `Auth` 验签失败时**不回显** expected；比较使用 `hmac.Equal`。测试 helper `fakeAuthFace`、`signHeader` 在此引入，供 Task 3 复用。

- [ ] **Step 1: 写失败测试（追加到 auth_test.go）**

```go
// fakeAuthFace 测试用 AuthFace 实现。
type fakeAuthFace struct {
	sk      string
	isDebug bool
	err     error
}

func (f *fakeAuthFace) GetSk(_ context.Context, _ *app.RequestContext, _ string, _ int64) (string, bool, error) {
	return f.sk, f.isDebug, f.err
}

// signHeader 构造 X-Signature 头：Base64(ak=..&sign=..&t=..)。
func signHeader(ak, sk, method, requestURI string, t int64, body []byte) string {
	sign := signRequest(ak, sk, method, requestURI, t, body)
	raw := fmt.Sprintf("ak=%s&sign=%s&t=%d", ak, sign, t)
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

func newAuthTestEngine(face AuthFace, opts ...Option) *route.Engine {
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(Auth(face, opts...))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.String(200, "ok")
	})
	engine.POST("/test", func(ctx context.Context, c *app.RequestContext) {
		c.String(200, "ok")
	})
	return engine
}

func TestAuth_ValidSignature_Passes(t *testing.T) {
	now := time.Now().Unix()
	engine := newAuthTestEngine(&fakeAuthFace{sk: "secret"})
	hdr := signHeader("ak1", "secret", "GET", "/test", now, nil)
	w := ut.PerformRequest(engine, "GET", "/test", nil,
		ut.Header{Key: "X-Signature", Value: hdr})
	assert.Equal(t, 200, w.Result().StatusCode())
}

func TestAuth_DebugDoesNotEchoServerSignature(t *testing.T) {
	now := time.Now().Unix()
	// 故意用错误 sk 生成签名
	face := &fakeAuthFace{sk: "secret", isDebug: true}
	engine := newAuthTestEngine(face)
	badHdr := signHeader("ak1", "wrong-sk", "GET", "/test", now, nil)
	w := ut.PerformRequest(engine, "GET", "/test", nil,
		ut.Header{Key: "X-Signature", Value: badHdr})

	res := w.Result()
	assert.Equal(t, 403, res.StatusCode(), "debug mode rejects with 403")
	body := string(res.Body())
	// 服务端正确签名绝不能出现在响应里
	expected := signRequest("ak1", "secret", "GET", "/test", now, nil)
	assert.NotContains(t, body, expected, "server signature must not be echoed")
	assert.NotContains(t, body, "server:", "legacy 'server:' leak must be gone")
}

func TestAuth_WrongSignature_NonDebug_401(t *testing.T) {
	now := time.Now().Unix()
	engine := newAuthTestEngine(&fakeAuthFace{sk: "secret"})
	badHdr := signHeader("ak1", "wrong-sk", "GET", "/test", now, nil)
	w := ut.PerformRequest(engine, "GET", "/test", nil,
		ut.Header{Key: "X-Signature", Value: badHdr})
	assert.Equal(t, 401, w.Result().StatusCode())
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd go-framework && go test ./hertz/middleware/ -run 'TestAuth_' -count=1`
Expected: **FAIL** —— `TestAuth_DebugDoesNotEchoServerSignature` 因当前代码回显 `server:%s` 而失败（响应含 expected）；比较仍为 `!=`。

- [ ] **Step 3: 修改 `Auth` —— 常量时间比较 + 移除回显**

替换 `Auth` 中验签比较与失败分支（约 `:47-55`）：

```go
		expected := signRequest(ak, sk, string(c.Request.Method()),
			string(c.Request.RequestURI()), t, c.Request.Body())
		if !hmac.Equal([]byte(expected), []byte(sign)) {
			if isDebug {
				c.AbortWithMsg("sign invalid", consts.StatusForbidden)
			} else {
				c.AbortWithStatus(consts.StatusUnauthorized)
			}
			return
		}
```

（移除原 `fmt.Sprintf("sign invalid, client:%s server:%s", ...)`；不再 import `fmt`。）

- [ ] **Step 4: 运行测试确认通过**

Run: `cd go-framework && go test ./hertz/middleware/ -run 'TestAuth_' -count=1`
Expected: **PASS**（3 个测试）。

- [ ] **Step 5: 提交**

```bash
git add go-framework/hertz/middleware/auth.go go-framework/hertz/middleware/auth_test.go
git commit -m "fix(go-framework): stop echoing valid signature in debug, use constant-time compare (#21)"
```

---

## Task 3: 时间戳新鲜度窗口（#23）+ Options + ParseInt 错误处理

**Files:**
- Modify: `go-framework/hertz/middleware/auth.go`（新增 Options/`timestampFresh`、修 `parseAuthorization`、`Auth` 加校验）
- Test: `go-framework/hertz/middleware/auth_test.go`

**Interfaces:**
- Consumes: `signRequest`（Task 1）、`fakeAuthFace`/`signHeader`/`newAuthTestEngine`（Task 2）
- Produces:
  - `const defaultTimestampWindow = 5 * time.Minute`
  - `type Option func(*authOptions)`；`func WithTimestampWindow(window time.Duration) Option`
  - `func timestampFresh(t int64, window time.Duration, now func() time.Time) bool`
  - `Auth(authFace AuthFace, opts ...Option) app.HandlerFunc`
  - `parseAuthorization` 在 `strconv.ParseInt` 失败时返回 error。

- [ ] **Step 1: 写失败测试（追加到 auth_test.go）**

```go
func TestTimestampFresh(t *testing.T) {
	now := func() time.Time { return time.Unix(1700000000, 0) }
	win := 5 * time.Minute

	assert.True(t, timestampFresh(1700000000, win, now), "exact now is fresh")
	assert.True(t, timestampFresh(1700000000-299, win, now), "within past window")
	assert.True(t, timestampFresh(1700000000+299, win, now), "within future window")
	assert.False(t, timestampFresh(1700000000-301, win, now), "expired past")
	assert.False(t, timestampFresh(1700000000+301, win, now), "too far future")
	assert.False(t, timestampFresh(0, win, now), "t=0 must be stale")
}

func TestAuth_ExpiredTimestamp_Rejected(t *testing.T) {
	stale := time.Now().Add(-10 * time.Minute).Unix()
	engine := newAuthTestEngine(&fakeAuthFace{sk: "secret"})
	hdr := signHeader("ak1", "secret", "GET", "/test", stale, nil)
	w := ut.PerformRequest(engine, "GET", "/test", nil,
		ut.Header{Key: "X-Signature", Value: hdr})
	assert.Equal(t, 401, w.Result().StatusCode())
}

func TestAuth_FutureTimestamp_Rejected(t *testing.T) {
	future := time.Now().Add(10 * time.Minute).Unix()
	engine := newAuthTestEngine(&fakeAuthFace{sk: "secret"})
	hdr := signHeader("ak1", "secret", "GET", "/test", future, nil)
	w := ut.PerformRequest(engine, "GET", "/test", nil,
		ut.Header{Key: "X-Signature", Value: hdr})
	assert.Equal(t, 401, w.Result().StatusCode())
}

func TestAuth_InvalidTimestamp_BadRequest(t *testing.T) {
	// t=abc 非法，parseAuthorization 应拒绝 → 400
	raw := base64.StdEncoding.EncodeToString([]byte("ak=ak1&sign=deadbeef&t=abc"))
	engine := newAuthTestEngine(&fakeAuthFace{sk: "secret"})
	w := ut.PerformRequest(engine, "GET", "/test", nil,
		ut.Header{Key: "X-Signature", Value: raw})
	assert.Equal(t, 400, w.Result().StatusCode())
}

func TestAuth_CustomTimestampWindow(t *testing.T) {
	// 窗口放大到 30min，则 10min 前的时间戳应被接受
	stale := time.Now().Add(-10 * time.Minute).Unix()
	engine := newAuthTestEngine(&fakeAuthFace{sk: "secret"}, WithTimestampWindow(30*time.Minute))
	hdr := signHeader("ak1", "secret", "GET", "/test", stale, nil)
	w := ut.PerformRequest(engine, "GET", "/test", nil,
		ut.Header{Key: "X-Signature", Value: hdr})
	assert.Equal(t, 200, w.Result().StatusCode())
}

func TestAuth_SignatureBindsBody_TamperRejected(t *testing.T) {
	now := time.Now().Unix()
	face := &fakeAuthFace{sk: "secret"}
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(Auth(face))
	engine.POST("/pay", func(ctx context.Context, c *app.RequestContext) { c.String(200, "ok") })

	// 客户端对 amount=1 签名
	hdr := signHeader("ak1", "secret", "POST", "/pay?amount=1", now, []byte(`{"a":1}`))
	// 重放到 amount=9999（query 篡改）
	w := ut.PerformRequest(engine, "POST", "/pay?amount=9999",
		&ut.Body{Body: []byte(`{"a":1}`)},
		ut.Header{Key: "X-Signature", Value: hdr})
	assert.Equal(t, 401, w.Result().StatusCode(), "query tamper must be rejected")
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd go-framework && go test ./hertz/middleware/ -run 'TestTimestampFresh|TestAuth_Expired|TestAuth_Future|TestAuth_Invalid|TestAuth_Custom|TestAuth_SignatureBindsBody' -count=1`
Expected: **编译失败**（`timestampFresh`/`WithTimestampWindow`/`Option` 未定义；`Auth` 仅 1 参）。

- [ ] **Step 3: 实现 Options + 窗口校验 + ParseInt 错误处理**

在 `auth.go` 顶部新增常量与 Options（紧跟 import 之后、`AuthFace` 之前或之后均可，保持分组注释）：

```go
// defaultTimestampWindow AK/SK 签名时间戳默认新鲜度窗口（±5 分钟）。
const defaultTimestampWindow = 5 * time.Minute

// authOptions Auth 中间件的可选配置。
type authOptions struct {
	timestampWindow time.Duration
}

// Option Auth 中间件配置选项函数。
type Option func(*authOptions)

// WithTimestampWindow 设置时间戳新鲜度窗口（±window）。
// window <= 0 时忽略，保持默认 5 分钟。
func WithTimestampWindow(window time.Duration) Option {
	return func(o *authOptions) {
		if window > 0 {
			o.timestampWindow = window
		}
	}
}
```

修改 `Auth` 签名与开头（应用 opts，并在 parse 成功后、`GetSk` 之前插入窗口校验）：

```go
// Auth 返回 Hertz AK/SK 鉴权中间件。
//
// 验证 X-Signature 头中的签名：Base64Decode(ak=xxx&sign=xxx&t=xxx)。
// 规范签名格式见 signRequest 文档。中间件强制时间戳新鲜度窗口（默认 ±5 分钟，
// 可用 WithTimestampWindow 配置），使用常量时间比较签名，验签失败不向客户端回显服务端签名。
func Auth(authFace AuthFace, opts ...Option) app.HandlerFunc {
	o := &authOptions{timestampWindow: defaultTimestampWindow}
	for _, opt := range opts {
		opt(o)
	}

	return func(ctx context.Context, c *app.RequestContext) {
		ak, sign, t, err := parseAuthorization(&c.Request)
		if err != nil {
			c.AbortWithStatus(consts.StatusBadRequest)
			return
		}

		if !timestampFresh(t, o.timestampWindow, time.Now) {
			c.AbortWithStatus(consts.StatusUnauthorized)
			return
		}

		sk, isDebug, err := authFace.GetSk(ctx, c, ak, t)
		if err != nil {
			if isDebug {
				c.AbortWithMsg(err.Error(), consts.StatusForbidden)
			} else {
				c.AbortWithStatus(consts.StatusUnauthorized)
			}
			return
		}

		expected := signRequest(ak, sk, string(c.Request.Method()),
			string(c.Request.RequestURI()), t, c.Request.Body())
		if !hmac.Equal([]byte(expected), []byte(sign)) {
			if isDebug {
				c.AbortWithMsg("sign invalid", consts.StatusForbidden)
			} else {
				c.AbortWithStatus(consts.StatusUnauthorized)
			}
			return
		}

		c.Next(ctx)
	}
}

// timestampFresh 判断 Unix 秒级时间戳 t 是否落在 ±window 新鲜度窗口内。
// now 通过参数注入以便测试；window <= 0 时回退默认窗口。
func timestampFresh(t int64, window time.Duration, now func() time.Time) bool {
	if window <= 0 {
		window = defaultTimestampWindow
	}
	diff := now().Unix() - t
	if diff < 0 {
		diff = -diff
	}
	return diff <= int64(window/time.Second)
}
```

修改 `parseAuthorization` 末尾的时间戳解析（不再吞错）：

```go
	tt, ok := kvs["t"]
	if !ok || tt == "" {
		return "", "", 0, goerror.In("auth.parseAuthorization").
			Code(goerror.CodeParamInvalid).
			Errorf("timestamp is empty")
	}
	parsed, perr := strconv.ParseInt(tt, 10, 64)
	if perr != nil {
		return "", "", 0, goerror.In("auth.parseAuthorization").
			Code(goerror.CodeParamInvalid).
			Wrap(perr)
	}
	t = parsed

	return ak, sign, t, nil
```

确保 import 含 `"time"`，且不再 import `"fmt"`。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd go-framework && go test ./hertz/middleware/ -count=1`
Expected: **PASS**（Task 1/2/3 全部测试）。

> **requestURI 一致性校验点：** `TestAuth_ValidSignature_Passes` 与 `TestAuth_SignatureBindsBody_TamperRejected` 依赖 `c.Request.RequestURI()` 返回 origin-form `path?query`。若 `ValidSignature_Passes` 因签名不匹配失败，将 `auth.go` 中 `string(c.Request.RequestURI())` 改为显式构造 `string(c.Request.URI().Path()) + "?" + string(c.Request.URI().QueryString())`（仅在 query 非空时拼接 `?`），再重跑。

- [ ] **Step 5: 提交**

```bash
git add go-framework/hertz/middleware/auth.go go-framework/hertz/middleware/auth_test.go
git commit -m "fix(go-framework): enforce configurable timestamp freshness window in AK/SK middleware (#23)"
```

---

## Task 4: 全量验证 + godoc 收尾

**Files:**
- Verify: `go-framework/...`

- [ ] **Step 1: gofmt + vet**

Run: `gofmt -l go-framework/hertz/middleware/auth.go go-framework/hertz/middleware/auth_test.go`
Expected: 无输出（已格式化）。
Run: `go vet ./hertz/middleware/`（在 `go-framework/` 下）
Expected: 无错误。

- [ ] **Step 2: 模块全量测试**

Run: `go test ./go-framework/... -count=1`（workspace 根）
Expected: **ok** 全部包。

- [ ] **Step 3: golangci-lint（v2）**

Run: `golangci-lint run --timeout=5m ./go-framework/...`
Expected: 无 issue（导出符号均有 godoc：`Auth`/`Option`/`WithTimestampWindow`/`AuthFace`）。

- [ ] **Step 4: 如有修复则提交**

```bash
git add -A && git commit -m "chore(go-framework): lint/gofmt fixes for AK/SK hardening"
```

---

## Self-Review

- **Spec 覆盖：** #21→Task 2（移除回显 + 常量时间比较 + 不泄漏测试）；#22→Task 1（method/query/body 覆盖）+ Task 3 的 body/query 篡改测试；#23→Task 3（窗口/Options/ParseInt 错误/过期·未来·非法时间戳测试）。godoc 更新随各 Task 落地。✅
- **Placeholder 扫描：** 无 TBD/TODO；所有代码步骤含完整代码与命令。requestURI 回退方案给出明确条件与代码。✅
- **类型一致性：** `signRequest(ak, sk, method, requestURI string, t int64, body []byte)` 在 Task 1/2/3 一致；`fakeAuthFace`/`signHeader`/`newAuthTestEngine` 在 Task 2 定义、Task 3 复用，签名一致；`timestampFresh(t int64, window time.Duration, now func() time.Time) bool` 一致。✅
- **越界检查：** 未触及 `example/`、`go-common/auth/ak.go`。✅
