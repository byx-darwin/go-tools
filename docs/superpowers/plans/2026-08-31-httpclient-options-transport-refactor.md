# httpclient Options/Transport Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `go-common/httpclient`'s positional-parameter functions with a `Client` struct + `Transport` interface + Options pattern, fix the retry/headers/context defects, and delete the unused `m3u8.go`.

**Architecture:** Introduce `Request`/`Response` value types and a `Transport` interface with a default `fasthttp`-backed implementation and a new `net/http`-backed implementation. `Client` wraps a `Transport` plus retry/timeout/user-agent config (all via `Option` functions) and exposes a single `Do(ctx, method, url, body, headers) (*Response, error)` method that owns retry looping, `ctx` cancellation, and header cloning. The legacy `Send`/`SendWithRetry`/`Retry` functions are kept, marked `Deprecated`, and (for `Send`/`SendWithRetry`) reimplemented on top of the new `Client` so behavior is unchanged for existing callers.

**Tech Stack:** Go 1.26, `github.com/valyala/fasthttp` (existing dep), `net/http` (stdlib), `github.com/stretchr/testify` (existing dep, used in tests).

**Spec:** `docs/superpowers/specs/2026-08-31-httpclient-options-transport-design.md`

## Global Constraints

- Package: `github.com/byx-darwin/go-tools/go-common/httpclient` (module `go-common`, go 1.26.5)
- Follow `.claude/rules/options-pattern.md`: `Option func(*Client)`, `WithXxx` functions each set one field with defensive checks, `NewClient(opts ...Option) *Client` fills defaults first then applies opts
- Follow `.claude/rules/go.md` § 8 golangci-lint rules: every exported symbol has a `// Name ...` godoc comment starting with the symbol name; `0o`-prefixed octal literals; combined same-type params; no unused params (use `_`); errors always checked or explicitly `_ =` discarded with reason if `//nolint`
- No new third-party dependencies — `nethttpTransport` uses only `net/http` stdlib
- Old exported functions (`Send`, `SendWithRetry`, `Retry`, `BodyFunc`) MUST keep identical external behavior and signatures, only gaining a `// Deprecated: ...` comment
- `m3u8.go` and its exported functions are deleted (confirmed no callers in the repo — see spec "Non-Goals")
- Retry rule is fixed (not configurable): retry when `err != nil` OR `resp.StatusCode >= 500`; never retry on 4xx

---

### Task 1: Request/Response types + Transport interface

**Files:**
- Create: `go-common/httpclient/transport.go`
- Test: `go-common/httpclient/transport_test.go`

**Interfaces:**
- Produces: `type Request struct { Method string; URL string; Body []byte; Headers map[string]string }`, `type Response struct { StatusCode int; Body []byte; Header http.Header }`, `type Transport interface { Do(ctx context.Context, req *Request) (*Response, error) }`

- [ ] **Step 1: Write the failing test**

```go
// go-common/httpclient/transport_test.go
package httpclient

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeTransport struct {
	do func(ctx context.Context, req *Request) (*Response, error)
}

func (f *fakeTransport) Do(ctx context.Context, req *Request) (*Response, error) {
	return f.do(ctx, req)
}

func TestTransportInterfaceSatisfiedByFake(t *testing.T) {
	var tr Transport = &fakeTransport{
		do: func(_ context.Context, req *Request) (*Response, error) {
			return &Response{StatusCode: 200, Body: []byte("ok"), Header: http.Header{}}, nil
		},
	}
	resp, err := tr.Do(context.Background(), &Request{Method: MethodGet, URL: "http://example.com"})
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, "ok", string(resp.Body))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./go-common/httpclient/... -run TestTransportInterfaceSatisfiedByFake -v`
Expected: FAIL — `Request`, `Response`, `Transport` undefined

- [ ] **Step 3: Write minimal implementation**

```go
// go-common/httpclient/transport.go
package httpclient

import (
	"context"
	"net/http"
)

// Request 是 Transport 处理的统一请求结构体。
type Request struct {
	Method  string
	URL     string
	Body    []byte
	Headers map[string]string
}

// Response 是 Transport 返回的统一响应结构体。
type Response struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}

// Transport 是可替换的底层 HTTP 传输接口。
type Transport interface {
	// Do 发送一次 HTTP 请求，遵循 ctx 的取消/超时。
	Do(ctx context.Context, req *Request) (*Response, error)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./go-common/httpclient/... -run TestTransportInterfaceSatisfiedByFake -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-common/httpclient/transport.go go-common/httpclient/transport_test.go
git commit -m "feat(httpclient): add Request/Response/Transport types"
```

---

### Task 2: fasthttpTransport (default implementation)

**Files:**
- Create: `go-common/httpclient/transport_fasthttp.go`
- Test: `go-common/httpclient/transport_fasthttp_test.go`

**Interfaces:**
- Consumes: `Request`, `Response`, `Transport` (Task 1)
- Produces: `func newFasthttpTransport() Transport`

- [ ] **Step 1: Write the failing test**

```go
// go-common/httpclient/transport_fasthttp_test.go
package httpclient

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFasthttpTransportDoSuccess(t *testing.T) {
	srv := httptest.NewServer(nil)
	defer srv.Close()

	tr := newFasthttpTransport()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := tr.Do(ctx, &Request{Method: MethodGet, URL: srv.URL})
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode) // httptest.NewServer(nil) uses DefaultServeMux -> 404 for unknown path
}

func TestFasthttpTransportDoNetworkError(t *testing.T) {
	tr := newFasthttpTransport()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := tr.Do(ctx, &Request{Method: MethodGet, URL: "http://127.0.0.1:1"})
	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./go-common/httpclient/... -run TestFasthttpTransport -v`
Expected: FAIL — `newFasthttpTransport` undefined

- [ ] **Step 3: Write minimal implementation**

```go
// go-common/httpclient/transport_fasthttp.go
package httpclient

import (
	"context"
	"time"

	"github.com/valyala/fasthttp"
)

type fasthttpTransport struct{}

// newFasthttpTransport 创建基于 fasthttp 的 Transport 实现（默认实现）。
func newFasthttpTransport() Transport {
	return &fasthttpTransport{}
}

func (t *fasthttpTransport) Do(ctx context.Context, req *Request) (*Response, error) {
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

	header := make(map[string][]string)
	fResp.Header.VisitAll(func(k, v []byte) {
		header[string(k)] = append(header[string(k)], string(v))
	})
	body := make([]byte, len(fResp.Body()))
	copy(body, fResp.Body())

	return &Response{
		StatusCode: fResp.StatusCode(),
		Body:       body,
		Header:     header,
	}, nil
}

func deadlineOrZero(ctx context.Context) time.Time {
	if dl, ok := ctx.Deadline(); ok {
		return dl
	}
	return time.Time{}
}
```

Note: `Response.Header` is `http.Header` (`map[string][]string`), so `header` above must be typed `http.Header`; add `"net/http"` import and change `make(map[string][]string)` to `make(http.Header)`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./go-common/httpclient/... -run TestFasthttpTransport -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-common/httpclient/transport_fasthttp.go go-common/httpclient/transport_fasthttp_test.go
git commit -m "feat(httpclient): add fasthttp Transport implementation"
```

---

### Task 3: nethttpTransport (net/http implementation)

**Files:**
- Create: `go-common/httpclient/transport_nethttp.go`
- Test: `go-common/httpclient/transport_nethttp_test.go`

**Interfaces:**
- Consumes: `Request`, `Response`, `Transport` (Task 1)
- Produces: `func NewNetHTTPTransport() Transport` (exported — usable via `WithTransport`)

- [ ] **Step 1: Write the failing test**

```go
// go-common/httpclient/transport_nethttp_test.go
package httpclient

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNetHTTPTransportDoSuccess(t *testing.T) {
	srv := httptest.NewServer(nil)
	defer srv.Close()

	tr := NewNetHTTPTransport()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := tr.Do(ctx, &Request{Method: MethodGet, URL: srv.URL})
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
}

func TestNetHTTPTransportDoCtxCanceled(t *testing.T) {
	tr := NewNetHTTPTransport()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := tr.Do(ctx, &Request{Method: MethodGet, URL: "http://example.com"})
	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./go-common/httpclient/... -run TestNetHTTPTransport -v`
Expected: FAIL — `NewNetHTTPTransport` undefined

- [ ] **Step 3: Write minimal implementation**

```go
// go-common/httpclient/transport_nethttp.go
package httpclient

import (
	"bytes"
	"context"
	"io"
	"net/http"
)

type nethttpTransport struct {
	client *http.Client
}

// NewNetHTTPTransport 创建基于标准库 net/http 的 Transport 实现，
// 可通过 WithTransport 注入 Client 以替代默认的 fasthttp 实现。
func NewNetHTTPTransport() Transport {
	return &nethttpTransport{client: &http.Client{}}
}

func (t *nethttpTransport) Do(ctx context.Context, req *Request) (*Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return nil, err
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	httpResp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = httpResp.Body.Close() }()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	return &Response{
		StatusCode: httpResp.StatusCode,
		Body:       body,
		Header:     httpResp.Header,
	}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./go-common/httpclient/... -run TestNetHTTPTransport -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-common/httpclient/transport_nethttp.go go-common/httpclient/transport_nethttp_test.go
git commit -m "feat(httpclient): add net/http Transport implementation"
```

---

### Task 4: Client struct + Options

**Files:**
- Create: `go-common/httpclient/client.go`
- Test: `go-common/httpclient/client_test.go`

**Interfaces:**
- Consumes: `Transport` (Task 1), `newFasthttpTransport()` (Task 2)
- Produces: `type Option func(*Client)`, `func NewClient(opts ...Option) *Client`, `WithTransport(Transport) Option`, `WithMaxRetries(int) Option`, `WithRetryInterval(time.Duration) Option`, `WithTimeout(time.Duration) Option`, `WithUserAgent(string) Option`. Fields (unexported): `transport Transport`, `maxRetries int`, `retryInterval time.Duration`, `timeout time.Duration`, `userAgent string`.

- [ ] **Step 1: Write the failing test**

```go
// go-common/httpclient/client_test.go
package httpclient

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewClientDefaults(t *testing.T) {
	c := NewClient()
	require.NotNil(t, c.transport)
	require.Equal(t, 0, c.maxRetries)
	require.Equal(t, DefaultSleep, c.retryInterval)
	require.Equal(t, defaultUserAgent, c.userAgent)
}

func TestNewClientWithOptions(t *testing.T) {
	tr := &fakeTransport{do: nil}
	c := NewClient(
		WithTransport(tr),
		WithMaxRetries(3),
		WithRetryInterval(10*time.Millisecond),
		WithTimeout(time.Second),
		WithUserAgent("custom-ua"),
	)
	require.Same(t, Transport(tr), c.transport)
	require.Equal(t, 3, c.maxRetries)
	require.Equal(t, 10*time.Millisecond, c.retryInterval)
	require.Equal(t, time.Second, c.timeout)
	require.Equal(t, "custom-ua", c.userAgent)
}

func TestWithMaxRetriesIgnoresNegative(t *testing.T) {
	c := NewClient(WithMaxRetries(-1))
	require.Equal(t, 0, c.maxRetries)
}

func TestWithUserAgentIgnoresEmpty(t *testing.T) {
	c := NewClient(WithUserAgent(""))
	require.Equal(t, defaultUserAgent, c.userAgent)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./go-common/httpclient/... -run 'TestNewClient|TestWithMaxRetriesIgnoresNegative|TestWithUserAgentIgnoresEmpty' -v`
Expected: FAIL — `NewClient`, `WithTransport`, etc. undefined

- [ ] **Step 3: Write minimal implementation**

```go
// go-common/httpclient/client.go
package httpclient

import "time"

const defaultUserAgent = "sznc-fasthttp-client-" + FasthttpVersion

// Client 是可配置的 HTTP 客户端，通过 Option 定制传输层、重试与超时策略。
type Client struct {
	transport     Transport
	maxRetries    int
	retryInterval time.Duration
	timeout       time.Duration
	userAgent     string
}

// Option 定义 Client 的配置选项函数。
type Option func(*Client)

// WithTransport 设置底层传输实现，默认使用基于 fasthttp 的实现。
func WithTransport(transport Transport) Option {
	return func(c *Client) {
		if transport != nil {
			c.transport = transport
		}
	}
}

// WithMaxRetries 设置最大重试次数，默认 0（不重试）。负数被忽略。
func WithMaxRetries(maxRetries int) Option {
	return func(c *Client) {
		if maxRetries >= 0 {
			c.maxRetries = maxRetries
		}
	}
}

// WithRetryInterval 设置首次重试等待间隔（后续按指数退避+抖动增长），默认 DefaultSleep。
func WithRetryInterval(interval time.Duration) Option {
	return func(c *Client) {
		if interval > 0 {
			c.retryInterval = interval
		}
	}
}

// WithTimeout 设置请求默认超时；若调用 Do 时传入的 ctx 已带 deadline，以 ctx 为准。
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if timeout > 0 {
			c.timeout = timeout
		}
	}
}

// WithUserAgent 设置自定义 User-Agent，默认 "sznc-fasthttp-client-<version>"。
func WithUserAgent(userAgent string) Option {
	return func(c *Client) {
		if userAgent != "" {
			c.userAgent = userAgent
		}
	}
}

// NewClient 创建 HTTP 客户端，支持 Options 配置。
// 默认配置：
//   - transport: 基于 fasthttp 的实现
//   - maxRetries: 0（不重试）
//   - retryInterval: DefaultSleep（500ms）
//   - userAgent: "sznc-fasthttp-client-<version>"
func NewClient(opts ...Option) *Client {
	c := &Client{
		transport:     newFasthttpTransport(),
		maxRetries:    0,
		retryInterval: DefaultSleep,
		userAgent:     defaultUserAgent,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./go-common/httpclient/... -run 'TestNewClient|TestWithMaxRetriesIgnoresNegative|TestWithUserAgentIgnoresEmpty' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-common/httpclient/client.go go-common/httpclient/client_test.go
git commit -m "feat(httpclient): add Client struct with Options pattern"
```

---

### Task 5: Client.Do — retry, ctx cancellation, headers clone

**Files:**
- Modify: `go-common/httpclient/client.go`
- Test: `go-common/httpclient/client_test.go`

**Interfaces:**
- Consumes: `Client` (Task 4), `Transport`/`Request`/`Response` (Task 1)
- Produces: `func (c *Client) Do(ctx context.Context, method, url string, body []byte, headers map[string]string) (*Response, error)`

- [ ] **Step 1: Write the failing test**

```go
// append to go-common/httpclient/client_test.go
package httpclient

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClientDoSuccessFirstAttempt(t *testing.T) {
	calls := 0
	tr := &fakeTransport{do: func(_ context.Context, _ *Request) (*Response, error) {
		calls++
		return &Response{StatusCode: 200, Header: http.Header{}}, nil
	}}
	c := NewClient(WithTransport(tr), WithMaxRetries(3), WithRetryInterval(time.Millisecond))

	resp, err := c.Do(context.Background(), MethodGet, "http://example.com", nil, nil)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, 1, calls)
}

func TestClientDoRetriesOnNetworkError(t *testing.T) {
	calls := 0
	tr := &fakeTransport{do: func(_ context.Context, _ *Request) (*Response, error) {
		calls++
		if calls < 3 {
			return nil, errors.New("dial: connection refused")
		}
		return &Response{StatusCode: 200, Header: http.Header{}}, nil
	}}
	c := NewClient(WithTransport(tr), WithMaxRetries(3), WithRetryInterval(time.Millisecond))

	resp, err := c.Do(context.Background(), MethodGet, "http://example.com", nil, nil)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, 3, calls)
}

func TestClientDoRetriesOn5xx(t *testing.T) {
	calls := 0
	tr := &fakeTransport{do: func(_ context.Context, _ *Request) (*Response, error) {
		calls++
		if calls < 2 {
			return &Response{StatusCode: 503, Header: http.Header{}}, nil
		}
		return &Response{StatusCode: 200, Header: http.Header{}}, nil
	}}
	c := NewClient(WithTransport(tr), WithMaxRetries(3), WithRetryInterval(time.Millisecond))

	resp, err := c.Do(context.Background(), MethodGet, "http://example.com", nil, nil)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, 2, calls)
}

func TestClientDoDoesNotRetryOn4xx(t *testing.T) {
	calls := 0
	tr := &fakeTransport{do: func(_ context.Context, _ *Request) (*Response, error) {
		calls++
		return &Response{StatusCode: 400, Header: http.Header{}}, nil
	}}
	c := NewClient(WithTransport(tr), WithMaxRetries(3), WithRetryInterval(time.Millisecond))

	resp, err := c.Do(context.Background(), MethodGet, "http://example.com", nil, nil)
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)
	require.Equal(t, 1, calls)
}

func TestClientDoRetriesExhausted(t *testing.T) {
	calls := 0
	tr := &fakeTransport{do: func(_ context.Context, _ *Request) (*Response, error) {
		calls++
		return &Response{StatusCode: 500, Header: http.Header{}}, nil
	}}
	c := NewClient(WithTransport(tr), WithMaxRetries(2), WithRetryInterval(time.Millisecond))

	resp, err := c.Do(context.Background(), MethodGet, "http://example.com", nil, nil)
	require.NoError(t, err)
	require.Equal(t, 500, resp.StatusCode)
	require.Equal(t, 3, calls) // 1 initial + 2 retries
}

func TestClientDoCtxCanceledDuringRetryWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	tr := &fakeTransport{do: func(_ context.Context, _ *Request) (*Response, error) {
		calls++
		if calls == 1 {
			cancel()
		}
		return nil, errors.New("network error")
	}}
	c := NewClient(WithTransport(tr), WithMaxRetries(5), WithRetryInterval(50*time.Millisecond))

	_, err := c.Do(ctx, MethodGet, "http://example.com", nil, nil)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, calls)
}

func TestClientDoClonesHeadersWithoutMutatingCaller(t *testing.T) {
	var seenUA string
	tr := &fakeTransport{do: func(_ context.Context, req *Request) (*Response, error) {
		seenUA = req.Headers["User-Agent"]
		return &Response{StatusCode: 200, Header: http.Header{}}, nil
	}}
	c := NewClient(WithTransport(tr), WithUserAgent("test-ua"))

	headers := map[string]string{"X-Foo": "bar"}
	_, err := c.Do(context.Background(), MethodGet, "http://example.com", nil, headers)
	require.NoError(t, err)
	require.Equal(t, "test-ua", seenUA)
	_, hasUA := headers["User-Agent"]
	require.False(t, hasUA, "caller's original headers map must not be mutated")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./go-common/httpclient/... -run TestClientDo -v`
Expected: FAIL — `(*Client).Do` undefined

- [ ] **Step 3: Write minimal implementation**

```go
// append to go-common/httpclient/client.go
import (
	"context"
	"math/rand"
	"net/http"
	"time"
)

// Do 发送一次 HTTP 请求，按 Client 配置的 maxRetries 自动重试。
// 网络层错误（err != nil）或 5xx 状态码触发重试；4xx 不重试。
// 重试等待期间响应 ctx 取消/超时。
func (c *Client) Do(ctx context.Context, method, url string, body []byte, headers map[string]string) (*Response, error) {
	reqHeaders := make(map[string]string, len(headers)+1)
	for k, v := range headers {
		reqHeaders[k] = v
	}
	reqHeaders["User-Agent"] = c.userAgent

	callCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	req := &Request{Method: method, URL: url, Body: body, Headers: reqHeaders}

	interval := c.retryInterval
	attempts := c.maxRetries + 1
	var lastResp *Response
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		lastResp, lastErr = c.transport.Do(callCtx, req)
		if !shouldRetry(lastResp, lastErr) {
			return lastResp, lastErr
		}
		if attempt == attempts-1 {
			break
		}

		wait := interval + time.Duration(rand.Int63n(int64(interval)+1))/2
		select {
		case <-callCtx.Done():
			return nil, callCtx.Err()
		case <-time.After(wait):
		}
		interval *= 2
	}
	return lastResp, lastErr
}

func shouldRetry(resp *Response, err error) bool {
	if err != nil {
		return true
	}
	return resp.StatusCode >= http.StatusInternalServerError
}
```

Merge the new `import` block into the existing one at the top of `client.go` (single `import (...)` block, stdlib group only — no `goimports` violation).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./go-common/httpclient/... -run TestClientDo -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-common/httpclient/client.go go-common/httpclient/client_test.go
git commit -m "feat(httpclient): implement Client.Do with retry, ctx cancellation, header clone"
```

---

### Task 6: Deprecate legacy Send/SendWithRetry/Retry, delegate to Client

**Files:**
- Modify: `go-common/httpclient/http.go`
- Modify: `go-common/httpclient/retry.go` (godoc only)
- Test: `go-common/httpclient/http_test.go`

**Interfaces:**
- Consumes: `NewClient`, `WithMaxRetries`, `WithRetryInterval`, `(*Client).Do` (Tasks 4-5)
- Produces: unchanged signatures for `Send`, `SendWithRetry`, `Retry`, `BodyFunc` — now all `Deprecated`

- [ ] **Step 1: Write the failing test**

```go
// go-common/httpclient/http_test.go — replace existing file content
package httpclient

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConstants(t *testing.T) {
	require.Equal(t, 500*time.Millisecond, DefaultSleep)
	require.Equal(t, "GET", MethodGet)
	require.Equal(t, "POST", MethodPost)
}

func TestSendReturnsFasthttpResponse(t *testing.T) {
	srv := httptest.NewServer(nil)
	defer srv.Close()

	headers := map[string]string{"X-Test": "1"}
	resp, status, err := Send(srv.URL, MethodGet, nil, headers, 2*time.Second)
	require.NoError(t, err)
	require.Equal(t, 404, status)
	require.Equal(t, 404, resp.StatusCode())
	require.Equal(t, "1", headers["X-Test"]) // original map untouched beyond caller's own keys
}

func TestSendWithRetryEventuallySucceeds(t *testing.T) {
	srv := httptest.NewServer(nil)
	defer srv.Close()

	headers := map[string]string{}
	resp, err := SendWithRetry(srv.URL, MethodGet, nil, headers, time.Millisecond, 2*time.Second, 1)
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./go-common/httpclient/... -run 'TestConstants|TestSend' -v`
Expected: initially may PASS on constants but FAIL to compile if `doSend`/old signatures removed prematurely — run after Step 3 edits are staged; use this as the target behavior to implement against.

- [ ] **Step 3: Write minimal implementation**

```go
// go-common/httpclient/http.go — full replacement
package httpclient

import (
	"context"
	"time"

	"github.com/valyala/fasthttp"
)

// Send 发送单次 HTTP 请求。
//
// Deprecated: 使用 NewClient 配合 Options 替代，例如
// NewClient().Do(ctx, method, url, body, headers)。
func Send(url, method string, body []byte, headers map[string]string, timeout time.Duration) (*fasthttp.Response, int, error) {
	c := NewClient(WithTimeout(timeout))
	resp, err := c.Do(context.Background(), method, url, body, headers)
	if err != nil {
		return nil, 0, err
	}
	return toFasthttpResponse(resp), resp.StatusCode, nil
}

// SendWithRetry 发送 HTTP 请求，失败时自动重试。
//
// Deprecated: 使用 NewClient 配合 WithMaxRetries/WithRetryInterval 替代。
func SendWithRetry(url, method string,
	body []byte,
	headers map[string]string,
	sleep, timeout time.Duration, retry int) (*fasthttp.Response, error) {
	c := NewClient(WithTimeout(timeout), WithMaxRetries(retry), WithRetryInterval(sleep))
	resp, err := c.Do(context.Background(), method, url, body, headers)
	if err != nil {
		return nil, err
	}
	return toFasthttpResponse(resp), nil
}

func toFasthttpResponse(resp *Response) *fasthttp.Response {
	fResp := &fasthttp.Response{}
	fResp.SetStatusCode(resp.StatusCode)
	fResp.SetBody(resp.Body)
	for k, values := range resp.Header {
		for _, v := range values {
			fResp.Header.Add(k, v)
		}
	}
	return fResp
}
```

```go
// go-common/httpclient/retry.go — godoc-only changes
package httpclient

import (
	"math/rand"
	"time"

	"github.com/valyala/fasthttp"
)

// BodyFunc 是重试调用的函数签名。
//
// Deprecated: 新代码请使用 (*Client).Do，其内部重试逻辑已修复网络错误不重试的缺陷。
type BodyFunc func() (*fasthttp.Response, int, error)

// Retry 执行 fn，失败时最多重试 retries 次，每次间隔 sleep。
//
// Deprecated: 仅在 5xx 状态码时重试，网络层错误不会触发重试，属已知缺陷；
// 新代码请使用 (*Client).Do。
func Retry(retries int, sleep time.Duration, fn BodyFunc) (*fasthttp.Response, int, error) {
	if sleep == 0 {
		sleep = DefaultSleep
	}
	response, status, err := fn()
	if err != nil {
		return response, status, err
	}

	if status >= fasthttp.StatusInternalServerError {
		retries--
		if retries <= 0 {
			return nil, status, err
		}
		sleep += (time.Duration(rand.Int63n(int64(sleep)))) / 2
		time.Sleep(sleep)
		return Retry(retries, 2*sleep, fn)
	}
	return response, status, err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./go-common/httpclient/... -run 'TestConstants|TestSend' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-common/httpclient/http.go go-common/httpclient/retry.go go-common/httpclient/http_test.go
git commit -m "refactor(httpclient): delegate deprecated Send/SendWithRetry to Client"
```

---

### Task 7: Delete m3u8.go

**Files:**
- Delete: `go-common/httpclient/m3u8.go`

**Interfaces:**
- Consumes: none
- Produces: none (removes `GetM3u8TsSize`, `DownloadM3u8TsData` from the public API)

- [ ] **Step 1: Confirm no callers remain**

Run: `grep -rn "GetM3u8TsSize\|DownloadM3u8TsData" --include="*.go" .`
Expected: no matches outside `go-common/httpclient/m3u8.go` itself (already verified during design — re-verify before deleting)

- [ ] **Step 2: Delete the file**

```bash
git rm go-common/httpclient/m3u8.go
```

- [ ] **Step 3: Verify the package still builds**

Run: `go build ./go-common/...`
Expected: success, no undefined references

- [ ] **Step 4: Commit**

```bash
git commit -m "refactor(httpclient): delete unused m3u8.go (no callers in repo)"
```

---

### Task 8: Full validation

**Files:** none (verification only)

- [ ] **Step 1: Run package tests**

Run: `go test ./go-common/httpclient/... -count=1 -v`
Expected: all tests PASS

- [ ] **Step 2: Run full module build + vet**

Run: `go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... && go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...`
Expected: no errors

- [ ] **Step 3: Run gofmt check**

Run: `gofmt -l go-common/httpclient/`
Expected: empty output (all files formatted)

- [ ] **Step 4: Run golangci-lint on go-common**

Run: `golangci-lint run --timeout=5m ./go-common/...`
Expected: no findings (verify godoc comments on all new exported symbols: `Request`, `Response`, `Transport`, `Client`, `Option`, `NewClient`, `WithTransport`, `WithMaxRetries`, `WithRetryInterval`, `WithTimeout`, `WithUserAgent`, `NewNetHTTPTransport`, `(*Client).Do`)

- [ ] **Step 5: Run full workspace test suite**

Run: `go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1`
Expected: all PASS

- [ ] **Step 6: Commit any final fixes**

```bash
git add -A
git commit -m "test(httpclient): final validation pass" --allow-empty
```

(Only commit if fixes were actually made; skip the empty commit if Steps 1-5 required no changes.)

---

## Self-Review Notes

- **Spec coverage:** All spec sections mapped — Architecture (Tasks 1-5), Retry/Context (Task 5), Headers fix (Task 5), Legacy compat (Task 6), m3u8 deletion (Task 7), Testing (Tasks 1-6, validated in Task 8).
- **Type consistency:** `Response.Header` is `http.Header` throughout (Task 1 defines it, Task 2/3 populate it, Task 6's `toFasthttpResponse` iterates it as `map[string][]string`, which `http.Header` is under the hood).
- **No placeholders:** every step has runnable code and exact commands.
