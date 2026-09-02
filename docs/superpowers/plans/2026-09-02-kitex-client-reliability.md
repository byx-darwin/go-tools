# Kitex 客户端可靠性配置 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `go-framework/kitex/option.NewClientOption` 实际消费 `CBSuite` 配置构造熔断 Option，为超时设置安全默认值，并为重试策略补充退避（backoff）配置。

**Architecture:** 修改两个既有文件：`go-framework/config/kitex/client.go`（新增 `FailureRetry.BackOff` 配置结构体）与 `go-framework/kitex/option/option.go`（新增包级默认超时常量、Functional Option `WithCircuitBreakerKeyFunc`、CBSuite 接线、退避策略接线）。不新增文件，不新增外部依赖（`circuitbreak`/`retry` 均已是 `cloudwego/kitex` 的既有子包）。

**Tech Stack:** Go 1.25（workspace），`github.com/cloudwego/kitex` v0.16.3（`pkg/circuitbreak`、`pkg/retry`、`pkg/rpcinfo`），`github.com/stretchr/testify`（assert/require），`oops`-based `frameworkerror`。

**Spec:** `docs/superpowers/specs/2026-09-02-kitex-client-reliability-design.md`

## Global Constraints

- 模块边界：改动仅限 `go-framework` 模块内（`go-framework/config/kitex` + `go-framework/kitex/option`），不得引入对 `go-middleware` 的依赖（CLAUDE.md DAG 约束）。
- Functional Options：新增可选参数须遵循 `.claude/rules/options-pattern.md`（`WithXxx` 命名、对无效输入做防御、不覆盖已有值、必须有 godoc 注释）。
- Lint：所有导出符号（类型/函数/常量/字段）必须有 `// Name ...` 格式 godoc 注释；`gofmt`/`goimports`（标准库/第三方/本项目三组）/`revive`/`errcheck`/`gocritic`/`misspell`/`unconvert`/`unparam` 全部通过；八进制字面量用 `0o` 写法（本次改动不涉及）。
- 向后兼容：`NewClientOption` 新增的 `opts ...Option` 为可变参数追加，不破坏现有调用签名；`FailureRetry.BackOff` 零值（`Type == ""`）必须保持现状行为（不启用退避）；`ConnectTimeout`/`RPCTimeout` 仍遵循"显式配置 `>0` 才覆盖默认值"的既有语义。
- TDD：每个 Task 先写失败测试，再实现，再验证通过，测试文件为 `go-framework/kitex/option/option_test.go`（`package option`，白盒可访问未导出符号）与 `go-framework/config/kitex/client_test.go`（`package kitex`）。
- 验证命令：`go build ./go-framework/... && go vet ./go-framework/... && go test ./go-framework/... -count=1`；`golangci-lint run --timeout=5m ./go-framework/...`（v2，见 `.claude/rules/go.md` §8.6）。

---

## File Structure

- **Modify** `go-framework/config/kitex/client.go` — 新增 `BackOff` 结构体，`FailureRetry` 新增 `BackOff` 字段。
- **Modify** `go-framework/config/kitex/client_test.go` — 补充 `BackOff` 字段相关断言。
- **Modify** `go-framework/kitex/option/option.go` — 新增默认超时常量、`Option`/`clientOptionConfig`/`WithCircuitBreakerKeyFunc`、`NewClientOption` 签名扩展与三处接线（超时默认值、CBSuite、退避策略）。
- **Modify** `go-framework/kitex/option/option_test.go` — 新增 `NewClientOption` 全套用例（当前文件只测 Server 侧）。

---

## Task 1: `FailureRetry.BackOff` 配置结构体

**Files:**
- Modify: `go-framework/config/kitex/client.go`
- Test: `go-framework/config/kitex/client_test.go`

**Interfaces:**
- Produces: `type BackOff struct { Type string; FixedMS int; MinMS int; MaxMS int }`；`FailureRetry.BackOff BackOff` 字段。后续 Task 4（`option.go`）依赖此结构体与字段名。

- [ ] **Step 1: 写失败测试**

在 `go-framework/config/kitex/client_test.go` 末尾追加：

```go
func TestFailureRetry_BackOffDefaults(t *testing.T) {
	fr := FailureRetry{}
	assert.Equal(t, "", fr.BackOff.Type)
	assert.Equal(t, 0, fr.BackOff.FixedMS)
	assert.Equal(t, 0, fr.BackOff.MinMS)
	assert.Equal(t, 0, fr.BackOff.MaxMS)
}

func TestFailureRetry_BackOffFixed(t *testing.T) {
	fr := FailureRetry{
		Enable:        true,
		MaxRetryTimes: 2,
		BackOff:       BackOff{Type: "fixed", FixedMS: 50},
	}
	assert.Equal(t, "fixed", fr.BackOff.Type)
	assert.Equal(t, 50, fr.BackOff.FixedMS)
}

func TestFailureRetry_BackOffRandom(t *testing.T) {
	fr := FailureRetry{
		Enable:        true,
		MaxRetryTimes: 2,
		BackOff:       BackOff{Type: "random", MinMS: 10, MaxMS: 100},
	}
	assert.Equal(t, "random", fr.BackOff.Type)
	assert.Equal(t, 10, fr.BackOff.MinMS)
	assert.Equal(t, 100, fr.BackOff.MaxMS)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-framework/config/kitex/... -run TestFailureRetry_BackOff -v`
Expected: FAIL（编译错误：`BackOff` / `fr.BackOff` 未定义）

- [ ] **Step 3: 实现**

在 `go-framework/config/kitex/client.go` 中，将 `FailureRetry` 结构体替换为：

```go
// FailureRetry 重试机制
type FailureRetry struct {
	Enable        bool    `json:"enable"  yaml:"enable"`
	MaxRetryTimes int     `json:"max_retry_times" yaml:"max_retry_times"`
	BackOff       BackOff `json:"backoff" yaml:"backoff"`
}

// BackOff 重试退避策略。Type 为空（零值）时保持不启用退避，向后兼容现有配置。
type BackOff struct {
	// Type 退避类型："" (不启用，默认) / "fixed" (固定退避) / "random" (随机区间退避)。
	Type string `json:"type" yaml:"type"`
	// FixedMS Type="fixed" 时的固定退避毫秒数，必须 > 0。
	FixedMS int `json:"fixed_ms" yaml:"fixed_ms"`
	// MinMS Type="random" 时的最小退避毫秒数。
	MinMS int `json:"min_ms" yaml:"min_ms"`
	// MaxMS Type="random" 时的最大退避毫秒数，必须大于 MinMS。
	MaxMS int `json:"max_ms" yaml:"max_ms"`
}
```

（插入位置：紧跟在现有 `FailureRetry` 结构体原位置，替换整个结构体定义；新增 `BackOff` 类型放在 `FailureRetry` 之后、`LoadBalancer` 之前。）

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-framework/config/kitex/... -v`
Expected: PASS（含既有的 `TestClientConfig_*` 用例，`golangci-lint run --timeout=5m ./go-framework/...` 中 `revive` 对新导出符号的注释检查通过）

- [ ] **Step 5: Commit**

```bash
git add go-framework/config/kitex/client.go go-framework/config/kitex/client_test.go
git commit -m "feat(kitex): add FailureRetry.BackOff config struct"
```

---

## Task 2: 超时默认值（ConnectTimeout / RPCTimeout）

**Files:**
- Modify: `go-framework/kitex/option/option.go`
- Test: `go-framework/kitex/option/option_test.go`

**Interfaces:**
- Consumes: `kitex.ClientConfig`、`kitex.ClientOption`、`kitex.ClientTimeout`（Task 1 未改动这部分，字段沿用现状：`RPCTimeout time.Duration`、`ConnectTimeOut time.Duration`）。
- Produces: 包级常量 `defaultConnectTimeout = 200 * time.Millisecond`、`defaultRPCTimeout = 3 * time.Second`。后续 Task 无依赖，但需与 Task 3/4 共存于同一文件的 `NewClientOption` 函数体内。

- [ ] **Step 1: 写失败测试**

在 `go-framework/kitex/option/option_test.go` 末尾追加（新增 `import "github.com/cloudwego/kitex/client"` 仅当需要引用类型时——本任务测试不需要引用 `client` 包，直接调用 `NewClientOption` 返回的 `[]client.Option` 长度即可，无需新增 import）：

```go
func TestNewClientOption_NilConfig(t *testing.T) {
	_, err := NewClientOption(t.Context(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client config is nil")
}

func TestNewClientOption_DefaultTimeouts(t *testing.T) {
	cfg := &kitex.ClientConfig{
		ClientOption: &kitex.ClientOption{},
	}
	opts, err := NewClientOption(t.Context(), cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, opts)
}

func TestNewClientOption_ExplicitTimeoutsOverrideDefaults(t *testing.T) {
	cfg := &kitex.ClientConfig{
		ClientOption: &kitex.ClientOption{
			Timeout: kitex.ClientTimeout{
				RPCTimeout:     10 * time.Second,
				ConnectTimeOut: 500 * time.Millisecond,
			},
		},
	}
	opts, err := NewClientOption(t.Context(), cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, opts)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-framework/kitex/option/... -run TestNewClientOption -v`
Expected: FAIL — `NewClientOption` 目前签名为 `(ctx, cfg)`，编译期这段代码其实已能通过（这是既有函数），所以本步针对的是"确认现状"：先运行确认当前测试通过（因为函数已存在），记录 baseline，再在 Step 3 中改动默认值实现并在 Step 4 复测。

若你严格要求"先失败后通过"的 TDD 闭环，可在 Step 1 的 `TestNewClientOption_DefaultTimeouts` 后追加一个直接断言默认值行为差异的用例（因 `[]client.Option` 是不透明的函数闭包，无法在黑盒断言中直接读出超时数值——这正是 Task 3 引入白盒 `clientOptionConfig` 模式前，本任务只能做"不 panic + 非空"的边界断言）。这里保留 Step 2 为"运行以确认现状可编译可跑"，不强制要求它先失败；重点在 Step 3 的默认值替换与 Step 4 的回归确认。

- [ ] **Step 3: 实现**

在 `go-framework/kitex/option/option.go` 顶部 `package option` 之后、`import` 块之后，新增包级常量：

```go
const (
	// defaultConnectTimeout ConnectTimeout 未显式配置时的默认值。
	defaultConnectTimeout = 200 * time.Millisecond
	// defaultRPCTimeout RPCTimeout 未显式配置时的默认值。
	defaultRPCTimeout = 3 * time.Second
)
```

将 `NewClientOption` 中原有的超时分支：

```go
	if co.Timeout.ConnectTimeOut > 0 {
		options = append(options, client.WithConnectTimeout(co.Timeout.ConnectTimeOut))
	} else {
		options = append(options, client.WithConnectTimeout(50*time.Millisecond))
	}

	if co.Timeout.RPCTimeout > 0 {
		options = append(options, client.WithRPCTimeout(co.Timeout.RPCTimeout))
	}
```

替换为：

```go
	if co.Timeout.ConnectTimeOut > 0 {
		options = append(options, client.WithConnectTimeout(co.Timeout.ConnectTimeOut))
	} else {
		options = append(options, client.WithConnectTimeout(defaultConnectTimeout))
	}

	if co.Timeout.RPCTimeout > 0 {
		options = append(options, client.WithRPCTimeout(co.Timeout.RPCTimeout))
	} else {
		options = append(options, client.WithRPCTimeout(defaultRPCTimeout))
	}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-framework/kitex/option/... -v`
Expected: PASS（含既有 Server 侧用例与本任务新增用例）

- [ ] **Step 5: Commit**

```bash
git add go-framework/kitex/option/option.go go-framework/kitex/option/option_test.go
git commit -m "fix(kitex): apply safe default RPCTimeout/ConnectTimeout when unconfigured"
```

---

## Task 3: CBSuite 接线 + `WithCircuitBreakerKeyFunc` Option

**Files:**
- Modify: `go-framework/kitex/option/option.go`
- Test: `go-framework/kitex/option/option_test.go`

**Interfaces:**
- Consumes: `kitex.ClientOption.CBSuite.Enable bool`（Task 1/2 未改动此字段，沿用现状）。
- Produces:
  - `type clientOptionConfig struct { cbKeyFunc circuitbreak.GenServiceCBKeyFunc }`（未导出，供 Task 4 若需要复用时参考，但 Task 4 不依赖它）。
  - `type Option func(*clientOptionConfig)`（导出）。
  - `func WithCircuitBreakerKeyFunc(f circuitbreak.GenServiceCBKeyFunc) Option`（导出）。
  - `NewClientOption` 签名变为 `func NewClientOption(ctx context.Context, cfg *kitex.ClientConfig, opts ...Option) ([]client.Option, error)`。Task 4 复用同一签名（不再变更）。

- [ ] **Step 1: 写失败测试**

在 `go-framework/kitex/option/option_test.go` 追加：

```go
func TestNewClientOption_CBSuiteDisabled(t *testing.T) {
	cfg := &kitex.ClientConfig{
		ClientOption: &kitex.ClientOption{
			CBSuite: kitex.CBSuite{Enable: false},
		},
	}
	optsDisabled, err := NewClientOption(t.Context(), cfg)
	require.NoError(t, err)

	cfg.ClientOption.CBSuite.Enable = true
	optsEnabled, err := NewClientOption(t.Context(), cfg)
	require.NoError(t, err)

	assert.Greater(t, len(optsEnabled), len(optsDisabled))
}

func TestWithCircuitBreakerKeyFunc_DefaultsToRPCInfo2Key(t *testing.T) {
	occ := &clientOptionConfig{cbKeyFunc: circuitbreak.RPCInfo2Key}
	assert.NotNil(t, occ.cbKeyFunc)
}

func TestWithCircuitBreakerKeyFunc_Custom(t *testing.T) {
	called := false
	custom := func(ri rpcinfo.RPCInfo) string {
		called = true
		return "custom-key"
	}

	occ := &clientOptionConfig{cbKeyFunc: circuitbreak.RPCInfo2Key}
	WithCircuitBreakerKeyFunc(custom)(occ)
	require.NotNil(t, occ.cbKeyFunc)

	got := occ.cbKeyFunc(nil)
	assert.True(t, called)
	assert.Equal(t, "custom-key", got)
}

func TestWithCircuitBreakerKeyFunc_NilIgnored(t *testing.T) {
	occ := &clientOptionConfig{cbKeyFunc: circuitbreak.RPCInfo2Key}
	before := reflect.ValueOf(occ.cbKeyFunc).Pointer()

	WithCircuitBreakerKeyFunc(nil)(occ)

	after := reflect.ValueOf(occ.cbKeyFunc).Pointer()
	assert.Equal(t, before, after)
}

func TestNewClientOption_CustomCBKeyFuncApplied(t *testing.T) {
	cfg := &kitex.ClientConfig{
		ClientOption: &kitex.ClientOption{
			CBSuite: kitex.CBSuite{Enable: true},
		},
	}
	custom := func(ri rpcinfo.RPCInfo) string { return "k" }

	opts, err := NewClientOption(t.Context(), cfg, WithCircuitBreakerKeyFunc(custom))
	require.NoError(t, err)
	assert.NotEmpty(t, opts)
}
```

在 `option_test.go` 顶部 `import` 块新增 `"reflect"` 与 `"github.com/cloudwego/kitex/pkg/circuitbreak"`、`"github.com/cloudwego/kitex/pkg/rpcinfo"`：

```go
import (
	"reflect"
	"testing"
	"time"

	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/byx-darwin/go-tools/go-framework/config/kitex"
)
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-framework/kitex/option/... -run "TestNewClientOption_CBSuite|TestWithCircuitBreakerKeyFunc|TestNewClientOption_CustomCBKeyFuncApplied" -v`
Expected: FAIL（编译错误：`clientOptionConfig`、`WithCircuitBreakerKeyFunc` 未定义）

- [ ] **Step 3: 实现**

在 `go-framework/kitex/option/option.go` 的 `import` 块中新增：

```go
	"github.com/cloudwego/kitex/pkg/circuitbreak"
```

（`rpcinfo` 已存在于现有 import 列表，无需重复添加。）

在 `// ── Client ──` 注释之后、`NewClientOption` 函数定义之前，新增：

```go
// clientOptionConfig 保存 NewClientOption 的可选配置（Functional Options）。
type clientOptionConfig struct {
	cbKeyFunc circuitbreak.GenServiceCBKeyFunc
}

// Option 定义 NewClientOption 的配置选项函数。
type Option func(*clientOptionConfig)

// WithCircuitBreakerKeyFunc 自定义熔断器 key 生成函数。
// 未设置（或传入 nil）时回退到 SDK 默认的 circuitbreak.RPCInfo2Key。
func WithCircuitBreakerKeyFunc(f circuitbreak.GenServiceCBKeyFunc) Option {
	return func(c *clientOptionConfig) {
		if f != nil {
			c.cbKeyFunc = f
		}
	}
}
```

将 `NewClientOption` 签名：

```go
func NewClientOption(ctx context.Context, cfg *kitex.ClientConfig) ([]client.Option, error) {
```

改为：

```go
func NewClientOption(ctx context.Context, cfg *kitex.ClientConfig, opts ...Option) ([]client.Option, error) {
```

在函数体 `co := cfg.ClientOption` 之后，新增：

```go
	occ := &clientOptionConfig{cbKeyFunc: circuitbreak.RPCInfo2Key}
	for _, opt := range opts {
		opt(occ)
	}
```

在 `if co.LoadBalancer.Enable { ... }` 代码块之后（`client.WithClientBasicInfo(...)` 之前）新增：

```go
	if co.CBSuite.Enable {
		options = append(options, client.WithCircuitBreaker(circuitbreak.NewCBSuite(occ.cbKeyFunc)))
	}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-framework/kitex/option/... -v`
Expected: PASS（全部用例，含 Task 2 新增用例）

- [ ] **Step 5: Commit**

```bash
git add go-framework/kitex/option/option.go go-framework/kitex/option/option_test.go
git commit -m "feat(kitex): wire CBSuite into NewClientOption with customizable key func"
```

---

## Task 4: 重试退避（Backoff）接线

**Files:**
- Modify: `go-framework/kitex/option/option.go`
- Test: `go-framework/kitex/option/option_test.go`

**Interfaces:**
- Consumes: `kitex.FailureRetry.BackOff`（Task 1 产出的 `BackOff{Type, FixedMS, MinMS, MaxMS}`）；`NewClientOption(ctx, cfg, opts ...Option) ([]client.Option, error)`（Task 3 产出的签名，本任务不再变更签名）。
- Produces: 无新导出符号；`NewClientOption` 在 `co.Failure.Enable` 分支内新增退避策略应用逻辑，并在配置非法时返回 `frameworkerror.ErrConfigInvalid` 包裹的 error。

- [ ] **Step 1: 写失败测试**

在 `go-framework/kitex/option/option_test.go` 追加：

```go
func TestNewClientOption_BackOffNone(t *testing.T) {
	cfg := &kitex.ClientConfig{
		ClientOption: &kitex.ClientOption{
			Failure: kitex.FailureRetry{Enable: true, MaxRetryTimes: 2},
		},
	}
	opts, err := NewClientOption(t.Context(), cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, opts)
}

func TestNewClientOption_BackOffFixed(t *testing.T) {
	cfg := &kitex.ClientConfig{
		ClientOption: &kitex.ClientOption{
			Failure: kitex.FailureRetry{
				Enable:        true,
				MaxRetryTimes: 2,
				BackOff:       kitex.BackOff{Type: "fixed", FixedMS: 50},
			},
		},
	}
	opts, err := NewClientOption(t.Context(), cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, opts)
}

func TestNewClientOption_BackOffFixedInvalid(t *testing.T) {
	cfg := &kitex.ClientConfig{
		ClientOption: &kitex.ClientOption{
			Failure: kitex.FailureRetry{
				Enable:        true,
				MaxRetryTimes: 2,
				BackOff:       kitex.BackOff{Type: "fixed", FixedMS: 0},
			},
		},
	}
	_, err := NewClientOption(t.Context(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fixed_ms")
}

func TestNewClientOption_BackOffRandom(t *testing.T) {
	cfg := &kitex.ClientConfig{
		ClientOption: &kitex.ClientOption{
			Failure: kitex.FailureRetry{
				Enable:        true,
				MaxRetryTimes: 2,
				BackOff:       kitex.BackOff{Type: "random", MinMS: 10, MaxMS: 100},
			},
		},
	}
	opts, err := NewClientOption(t.Context(), cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, opts)
}

func TestNewClientOption_BackOffRandomInvalid(t *testing.T) {
	cfg := &kitex.ClientConfig{
		ClientOption: &kitex.ClientOption{
			Failure: kitex.FailureRetry{
				Enable:        true,
				MaxRetryTimes: 2,
				BackOff:       kitex.BackOff{Type: "random", MinMS: 100, MaxMS: 10},
			},
		},
	}
	_, err := NewClientOption(t.Context(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max_ms")
}

func TestNewClientOption_BackOffUnknownType(t *testing.T) {
	cfg := &kitex.ClientConfig{
		ClientOption: &kitex.ClientOption{
			Failure: kitex.FailureRetry{
				Enable:        true,
				MaxRetryTimes: 2,
				BackOff:       kitex.BackOff{Type: "exponential"},
			},
		},
	}
	_, err := NewClientOption(t.Context(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backoff")
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-framework/kitex/option/... -run TestNewClientOption_BackOff -v`
Expected: FAIL（`TestNewClientOption_BackOffFixedInvalid` 等断言 `require.Error` 失败，因为当前实现遇到非法 `FixedMS=0` 时会直接 panic 于 `retry.FailurePolicy.WithFixedBackOff`，而不是返回 error）

- [ ] **Step 3: 实现**

在 `go-framework/kitex/option/option.go` 的 `import` 块中新增 `"fmt"`：

```go
import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
	...
```

将 `NewClientOption` 中原有的重试分支：

```go
	if co.Failure.Enable {
		fp := retry.NewFailurePolicy()
		fp.WithMaxRetryTimes(co.Failure.MaxRetryTimes)
		options = append(options, client.WithFailureRetry(fp))
	}
```

替换为：

```go
	if co.Failure.Enable {
		fp := retry.NewFailurePolicy()
		fp.WithMaxRetryTimes(co.Failure.MaxRetryTimes)

		switch co.Failure.BackOff.Type {
		case "":
			// 不启用退避，保持现状行为
		case "fixed":
			if co.Failure.BackOff.FixedMS <= 0 {
				return nil, frameworkerror.ErrConfigInvalid.With("step", "backoff_fixed").Wrap(
					fmt.Errorf("failure_retry.backoff.fixed_ms must be > 0, got %d", co.Failure.BackOff.FixedMS))
			}
			fp.WithFixedBackOff(co.Failure.BackOff.FixedMS)
		case "random":
			if co.Failure.BackOff.MaxMS <= co.Failure.BackOff.MinMS {
				return nil, frameworkerror.ErrConfigInvalid.With("step", "backoff_random").Wrap(
					fmt.Errorf("failure_retry.backoff.max_ms(%d) must be > min_ms(%d)",
						co.Failure.BackOff.MaxMS, co.Failure.BackOff.MinMS))
			}
			fp.WithRandomBackOff(co.Failure.BackOff.MinMS, co.Failure.BackOff.MaxMS)
		default:
			return nil, frameworkerror.ErrConfigInvalid.With("step", "backoff_type").Wrap(
				fmt.Errorf("unknown failure_retry.backoff.type %q", co.Failure.BackOff.Type))
		}

		options = append(options, client.WithFailureRetry(fp))
	}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-framework/kitex/option/... -v`
Expected: PASS（全部用例）

- [ ] **Step 5: Commit**

```bash
git add go-framework/kitex/option/option.go go-framework/kitex/option/option_test.go
git commit -m "feat(kitex): add retry backoff wiring with config validation"
```

---

## Task 5: 全量验证 + 设计文档随分支提交

**Files:**
- No new code changes; validation only.
- Commit: `docs/superpowers/specs/2026-09-02-kitex-client-reliability-design.md`（Phase 1 已写入本地但未提交，随实现分支一并提交）

**Interfaces:**
- Consumes: Task 1-4 的全部实现。
- Produces: 无新符号；本任务是收尾验证 + 文档提交。

- [ ] **Step 1: 全量构建**

Run: `go build ./go-framework/...`
Expected: 无错误

- [ ] **Step 2: 全量 vet**

Run: `go vet ./go-framework/...`
Expected: 无错误

- [ ] **Step 3: 全量测试**

Run: `go test ./go-framework/... -count=1`
Expected: PASS

- [ ] **Step 4: Lint**

Run: `golangci-lint run --timeout=5m ./go-framework/...`
Expected: 0 issues（若版本非 v2 报错，参考 `.claude/rules/go.md` §8.6 升级）

- [ ] **Step 5: gofmt 检查**

Run: `gofmt -l go-framework/config/kitex/client.go go-framework/config/kitex/client_test.go go-framework/kitex/option/option.go go-framework/kitex/option/option_test.go`
Expected: 无输出（全部文件已格式化）

- [ ] **Step 6: 提交设计文档**

```bash
git add docs/superpowers/specs/2026-09-02-kitex-client-reliability-design.md docs/superpowers/plans/2026-09-02-kitex-client-reliability.md
git commit -m "docs: add design + implementation plan for kitex client reliability (#93)"
```

---

## Self-Review Notes（写作时已核对，供执行者参考）

- **Spec 覆盖**：CBSuite 接线（Task 3）、超时默认值（Task 2）、重试退避（Task 1 + Task 4）三项设计要点均有对应任务；`WithCircuitBreakerKeyFunc` 自定义 key 函数（评审补充要求）已纳入 Task 3。
- **类型一致性**：`kitex.BackOff{Type, FixedMS, MinMS, MaxMS}`（Task 1 定义）与 Task 4 中 `co.Failure.BackOff.*` 字段名一致；`clientOptionConfig.cbKeyFunc circuitbreak.GenServiceCBKeyFunc`（Task 3 定义）与 `NewClientOption` 内部使用一致；`NewClientOption(ctx, cfg, opts ...Option)` 签名在 Task 3 引入后，Task 4 不再变更。
- **向后兼容核查**：`BackOff.Type == ""` 走 `case ""`（no-op）分支，不改变现有已启用重试但未配置退避的调用方行为；`NewClientOption` 新增的 `opts ...Option` 是可变参数追加，仓库内无现存非测试调用点（已用 grep 确认），不存在破坏性影响。
