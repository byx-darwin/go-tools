# fasthttpTransport ctx 已过期修复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 `fasthttpTransport.Do` 在 ctx 已取消/已过期时仍发起无超时网络请求的问题，使其行为与 `nethttpTransport` 一致。

**Architecture:** 在 `fasthttpTransport.Do` 入口处增加一次 `ctx.Err()` 检查；非 nil 时直接返回该错误，不获取 fasthttp 请求/响应对象、不发起网络调用。原有 `timeout > 0`（有效 deadline，走 `fasthttp.DoTimeout`）与 `else`（无 deadline，走 `fasthttp.Do`）分支保持不变。

**Tech Stack:** Go, `github.com/valyala/fasthttp`, `testify/require`

**Spec:** `docs/superpowers/specs/2026-08-31-fasthttp-transport-ctx-expired-fix-design.md`

## Global Constraints

- 保持 `nethttpTransport` 行为不变（不在本次修改范围内）
- 无 deadline 的 ctx 继续走无超时的 `fasthttp.Do`（不引入行为变化）
- 新增代码需通过项目 `golangci-lint` 规则（godoc 注释、errcheck 等，见 `.claude/rules/go.md`）

---

### Task 1: fasthttpTransport ctx 早退检查 + 单元测试

**Files:**
- Modify: `go-common/httpclient/transport_fasthttp.go`
- Test: `go-common/httpclient/transport_fasthttp_test.go`

**Interfaces:**
- Consumes: `fasthttpTransport.Do(ctx context.Context, req *Request) (*Response, error)` — 现有方法签名不变
- Produces: 无新增导出符号

- [ ] **Step 1: Write the failing test — 已取消 ctx**

在 `go-common/httpclient/transport_fasthttp_test.go` 追加：

```go
func TestFasthttpTransportDoCanceledContext(t *testing.T) {
	tr := newFasthttpTransport()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := tr.Do(ctx, &Request{Method: MethodGet, URL: "http://127.0.0.1:1"})
	require.ErrorIs(t, err, context.Canceled)
}
```

- [ ] **Step 2: Write the failing test — 已过期 deadline ctx**

同一文件追加：

```go
func TestFasthttpTransportDoExpiredDeadlineContext(t *testing.T) {
	tr := newFasthttpTransport()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Hour))
	defer cancel()

	_, err := tr.Do(ctx, &Request{Method: MethodGet, URL: "http://127.0.0.1:1"})
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
```

**Note:** 两个测试都使用 `http://127.0.0.1:1`（不可达地址）——如果代码错误地发起了网络请求，会返回连接错误而非 `context.Canceled`/`context.DeadlineExceeded`，从而使 `require.ErrorIs` 断言失败，这就是验证"未发起网络调用"的方式。

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./go-common/httpclient/... -run 'TestFasthttpTransportDoCanceledContext|TestFasthttpTransportDoExpiredDeadlineContext' -v
```

Expected: 两个测试都 FAIL（当前实现会尝试连接 `127.0.0.1:1` 并返回连接被拒绝错误，而不是 ctx 错误）。

- [ ] **Step 4: Write minimal implementation**

修改 `go-common/httpclient/transport_fasthttp.go` 中的 `Do` 方法，在方法体最前面增加早退检查：

```go
func (t *fasthttpTransport) Do(ctx context.Context, req *Request) (*Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	fReq := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(fReq)
	fReq.Header.SetMethod(req.Method)
	fReq.SetRequestURI(req.URL)
	for k, v := range req.Headers {
		fReq.Header.Set(k, v)
	}
	if len(req.Body) > 0 {
		fReq.SetBody(req.Body)
	}

	fResp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(fResp)

	timeout := time.Until(deadlineOrZero(ctx))
	var err error
	if timeout > 0 {
		err = fasthttp.DoTimeout(fReq, fResp, timeout)
	} else {
		err = fasthttp.Do(fReq, fResp)
	}
	if err != nil {
		return nil, err
	}

	header := make(http.Header)
	for k, v := range fResp.Header.All() {
		header[string(k)] = append(header[string(k)], string(v))
	}
	body := make([]byte, len(fResp.Body()))
	copy(body, fResp.Body())

	return &Response{
		StatusCode: fResp.StatusCode(),
		Body:       body,
		Header:     header,
	}, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./go-common/httpclient/... -run 'TestFasthttpTransportDoCanceledContext|TestFasthttpTransportDoExpiredDeadlineContext' -v
```

Expected: PASS

- [ ] **Step 6: Run full package test suite to check for regressions**

```bash
go test ./go-common/httpclient/... -count=1 -v
```

Expected: 所有测试（包括 `TestFasthttpTransportDoSuccess`、`TestFasthttpTransportDoNetworkError`）均 PASS。

- [ ] **Step 7: Lint check**

```bash
gofmt -l go-common/httpclient/transport_fasthttp.go go-common/httpclient/transport_fasthttp_test.go
go vet ./go-common/httpclient/...
golangci-lint run --timeout=5m ./go-common/...
```

Expected: 无输出（gofmt）、无错误（vet / lint）。

- [ ] **Step 8: Commit**

```bash
git add go-common/httpclient/transport_fasthttp.go go-common/httpclient/transport_fasthttp_test.go
git commit -m "fix(go-common/httpclient): fasthttpTransport 立即返回已取消/已过期 ctx 的错误

Closes #64"
```
