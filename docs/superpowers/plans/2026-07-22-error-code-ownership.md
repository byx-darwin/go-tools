# 错误码归属迁移（#27）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将错误码定义从 `go-common/error` 迁回属主模块（方案 a），使上层模块新增错误码不再需要 go-common 发版，解除对 D4 独立版本化的威胁。

**Architecture:** `go-common/error` 瘦身为纯机制包（构造/提取函数 + 码段边界常量 + HTTP 状态注册表）；`go-framework/error`（新包 `frameworkerror`）持有 10000 段与 Obs 20601–20605 段；`go-middleware/{clickhouse,tls}` 各持有本包错误码。细粒度 HTTP 状态映射由各模块 `init()` 注册到 go-common 注册表，`HTTPStatus()` 先查注册表再走范围兜底。码值一律不变（wire 稳定），62 个死码符号删除，不加 `Deprecated` 别名。

**Tech Stack:** Go 1.25+（workspace go.work，go 1.26.5 指令）、samber/oops、testify、golangci-lint v2。

## Global Constraints

- **码值不可变更**：所有迁移的码常量保持原数值（10000–10013、20401–20403、20501–20504、20601–20605）与 public 消息字符串不变。
- **`ProjectCodeMin` 保持 40000**：原样搬迁，修正为 40100 + 新增 `AuthCodeMin/Max` 属于 #28，本计划禁止改动。
- **禁止 `Deprecated` 别名层**：直接移动/删除符号，调用方同 PR 内更新。
- **禁止预建 Redis/Kafka/DB/ES 错误码**（YAGNI）。
- **lint 必须逐模块运行**（golangci-lint 不支持 workspace 根 `./...`）：`for m in go-common go-auth go-middleware go-framework; do golangci-lint run --timeout=5m ./$m/... || exit 1; done`，版本 v2（≥ v2.12.2）。
- **revive**：所有导出符号必须有 `// Name ...` 格式 godoc；**errcheck**：error 必须处理；**gocritic**：`0o644` 八进制、`paramTypeCombine`、`//nolint:xxx // 原因`；**goimports**：标准库/第三方/本项目三组。
- **提交前缀**：`feat(go-common)` / `feat(go-framework)` / `feat(go-middleware)` / `refactor(...)` / `docs`；每条 commit message 尾部带 `(#27)`。
- **TDD**：每个行为变更任务先写失败测试，再写实现；每个任务以独立 commit 收尾。
- **PR**：`Closes #27, Refs #34`；设计文档随分支提交。

## File Structure

| 文件 | 职责 | 动作 |
|------|------|------|
| `go-common/error/httpstatus.go` | HTTP 状态注册表（RegisterHTTPStatuses + lookup） | 新建（Task 1） |
| `go-common/error/httpstatus_test.go` | 注册表测试 | 新建（Task 1） |
| `go-common/error/error.go` | 瘦身后机制包 | 改（Task 1 加注册表优先；Task 6 删符号+删 switch） |
| `go-common/error/error_test.go` | 机制测试 | 改（Task 6 重整） |
| `go-framework/error/error.go` | frameworkerror：10000 段 + obs 20601–20605 + init() 注册 | 新建（Task 2） |
| `go-framework/error/error_test.go` | 码值/构造/HTTP 映射测试 | 新建（Task 2） |
| `go-middleware/clickhouse/errors.go` | CH 20401–20403 + init() 注册 | 新建（Task 3） |
| `go-middleware/clickhouse/errors_test.go` | CH 错误测试 | 新建（Task 3） |
| `go-middleware/tls/errors.go` | TLS 20501–20504 + init() 注册 | 新建（Task 3） |
| `go-middleware/tls/errors_test.go` | TLS 错误测试 | 新建（Task 3） |
| `go-middleware/clickhouse/client.go` | 改用包内 `ErrParseDSN` | 改（Task 4） |
| `go-middleware/tls/producer.go` | 改用包内 `CodeInvalidConfig`/`CodeSend` | 改（Task 4） |
| `go-middleware/tls/shipper.go` | 改用包内 `CodeInvalidConfig`/`CodeProducerInit` | 改（Task 4） |
| `go-framework/hertz/register.go` | blank-import frameworkerror 保证注册 | 新建（Task 5） |
| `go-framework/config/polaris.go`、`hertz/middleware/auth.go`、`hertz/observability/provider.go`、`kitex/observability/provider.go`、`kitex/option/option.go`、`kitex/rpcerror/error.go` | 调用方迁移至 frameworkerror | 改（Task 5） |
| `go-framework/hertz/response_test.go`、`response_integration_test.go`、`kitex/rpcerror/error_test.go` | 测试迁移 | 改（Task 5） |
| `example/handler/common_error.go` | `ErrParamInvalid` → frameworkerror | 改（Task 6） |
| `specs/00_overview.md`、`CLAUDE.md`、`go-middleware/README.md` | 归属模型文档（D6） | 改（Task 7） |

---

### Task 1: 准备分支 + go-common/error HTTP 状态注册表（TDD）

**Files:**
- Create: `go-common/error/httpstatus.go`
- Create: `go-common/error/httpstatus_test.go`
- Modify: `go-common/error/error.go`（`HTTPStatus` 先查注册表）
- Commit（docs）: `docs/superpowers/specs/2026-07-22-error-code-ownership-design.md`、`docs/superpowers/plans/2026-07-22-error-code-ownership.md`

**Interfaces:**
- Produces: `func RegisterHTTPStatuses(m map[int]int)`（重复注册同一 code → panic）、内部 `lookupHTTPStatus(code int) (int, bool)`。Task 2/3 的 `init()` 依赖此 API。

- [ ] **Step 1: 提交设计文档与实施计划**

确认 worktree 分支 `feat/27-error-code-ownership` 已由编排器创建。将主仓库工作区中未跟踪的两个文档复制进 worktree 并提交：

```bash
# 在 worktree 根目录执行（<MAIN> 为主仓库检出路径）
mkdir -p docs/superpowers/specs docs/superpowers/plans
cp <MAIN>/docs/superpowers/specs/2026-07-22-error-code-ownership-design.md docs/superpowers/specs/
cp <MAIN>/docs/superpowers/plans/2026-07-22-error-code-ownership.md docs/superpowers/plans/
git add docs/superpowers/specs/2026-07-22-error-code-ownership-design.md docs/superpowers/plans/2026-07-22-error-code-ownership.md
git commit -m "docs: add error-code ownership design & implementation plan (#27)"
```

- [ ] **Step 2: 写失败测试 `go-common/error/httpstatus_test.go`**

```go
package error

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// 测试专用码使用 39900-39989 未分配段，避免与各模块 init() 注册的真实码冲突。

func TestRegisterHTTPStatuses_Lookup(t *testing.T) {
	RegisterHTTPStatuses(map[int]int{39901: 418})
	err := Code(39901).Public("teapot").Wrap(errors.New("x"))
	assert.Equal(t, 418, HTTPStatus(err))
}

func TestRegisterHTTPStatuses_RegistryPrecedence(t *testing.T) {
	// 未注册时 39903 走兜底（迁移期内置 switch default → 200）；注册后应返回注册值。
	RegisterHTTPStatuses(map[int]int{39903: 503})
	err := Code(39903).Public("svc_down").Wrap(errors.New("x"))
	assert.Equal(t, 503, HTTPStatus(err))
}

func TestRegisterHTTPStatuses_DuplicatePanics(t *testing.T) {
	RegisterHTTPStatuses(map[int]int{39902: 500})
	assert.Panics(t, func() {
		RegisterHTTPStatuses(map[int]int{39902: 503})
	})
}

func TestIsClientError_RegisteredCode(t *testing.T) {
	RegisterHTTPStatuses(map[int]int{39905: 404})
	assert.True(t, IsClientError(39905))
}

func TestIsServerError_RegisteredCode(t *testing.T) {
	RegisterHTTPStatuses(map[int]int{39906: 503})
	assert.True(t, IsServerError(39906))
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./go-common/error/ -run 'TestRegisterHTTPStatuses|TestIsClientError_Registered|TestIsServerError_Registered' -count=1`
Expected: FAIL — `undefined: RegisterHTTPStatuses`

- [ ] **Step 4: 实现注册表 `go-common/error/httpstatus.go`**

```go
package error

import (
	"fmt"
	"sync"
)

// httpStatusRegistry 存储各模块注册的"错误码 → HTTP 状态码"细粒度映射。
// 仅预期在包 init() 阶段写入，运行期只读。
var (
	httpStatusMu       sync.RWMutex
	httpStatusRegistry = map[int]int{}
)

// RegisterHTTPStatuses 注册错误码到 HTTP 状态码的细粒度映射。
// 预期在各模块的包 init() 中调用（如 go-framework/error、go-middleware/clickhouse）。
// 重复注册同一错误码会 panic，以在启动期暴露配置错误。
func RegisterHTTPStatuses(m map[int]int) {
	httpStatusMu.Lock()
	defer httpStatusMu.Unlock()
	for code, status := range m {
		if _, exists := httpStatusRegistry[code]; exists {
			panic(fmt.Sprintf("go-common/error: duplicate HTTP status registration for code %d", code))
		}
		httpStatusRegistry[code] = status
	}
}

// lookupHTTPStatus 查询错误码的已注册 HTTP 状态码。
func lookupHTTPStatus(code int) (int, bool) {
	httpStatusMu.RLock()
	defer httpStatusMu.RUnlock()
	status, ok := httpStatusRegistry[code]
	return status, ok
}
```

- [ ] **Step 5: 修改 `go-common/error/error.go`（注册表优先，三个判定函数统一走 httpStatusForCode）**

将现有：

```go
// HTTPStatus 从 error 中提取错误码，映射为 HTTP 状态码。
func HTTPStatus(err error) int {
	code, _ := Extract(err)
	return httpStatusByCode(code)
}
```

替换为：

```go
// HTTPStatus 从 error 中提取错误码，映射为 HTTP 状态码。
// 优先级：各模块注册的细粒度映射 → 内置映射（迁移期保留，Task 6 由范围兜底替代）。
func HTTPStatus(err error) int {
	code, _ := Extract(err)
	return httpStatusForCode(code)
}

// httpStatusForCode 按注册表 + 内置 switch 映射错误码到 HTTP 状态码。
// 迁移期内置 switch 作为二级兜底；Task 6 删除 switch 后改为范围兜底。
func httpStatusForCode(code int) int {
	if status, ok := lookupHTTPStatus(code); ok {
		return status
	}
	return httpStatusByCode(code)
}
```

并将 `IsClientError` / `IsServerError` 改为经由 `httpStatusForCode`（否则二者绕过注册表）：

```go
// IsClientError 判断错误码是否属于客户端错误（4xx）。
func IsClientError(code int) bool {
	s := httpStatusForCode(code)
	return s >= 400 && s < 500
}

// IsServerError 判断错误码是否属于服务端/基础设施错误（5xx）。
func IsServerError(code int) bool {
	return httpStatusForCode(code) >= 500
}
```

- [ ] **Step 6: 运行全部 go-common/error 测试**

Run: `go test ./go-common/error/ -count=1`
Expected: PASS（新测试 + 现有 `error_test.go` 全绿——注册表优先不影响未注册码的既有行为）

- [ ] **Step 7: 提交**

```bash
gofmt -w go-common/error/httpstatus.go go-common/error/httpstatus_test.go go-common/error/error.go
git add go-common/error/httpstatus.go go-common/error/httpstatus_test.go go-common/error/error.go
git commit -m "feat(go-common): add HTTP status registry with duplicate-panic registration (#27)"
```

---

### Task 2: go-framework/error 包（frameworkerror，TDD）

**Files:**
- Create: `go-framework/error/error.go`
- Create: `go-framework/error/error_test.go`

**Interfaces:**
- Consumes: `goerror.Code(any) Builder`、`goerror.RegisterHTTPStatuses(map[int]int)`（Task 1）
- Produces: 常量 `CodeSystem…CodeRPCEncodeError`（10000–10013）、`CodeObsInit…CodeObsRuntimeMetrics`（20601–20605）；构造器 `ErrSystem…ErrRPCEncodeError`、`ErrObsInit…ErrObsRuntimeMetrics`；`init()` 注册 HTTP 映射。Task 5/6 的调用方迁移依赖这些符号。

- [ ] **Step 1: 写失败测试 `go-framework/error/error_test.go`**

```go
package frameworkerror_test

import (
	"errors"
	"testing"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
	"github.com/stretchr/testify/assert"
)

// TestCodeValues 码值是 wire 契约，逐值锁定（禁止改号）。
func TestCodeValues(t *testing.T) {
	assert.Equal(t, 10000, frameworkerror.CodeSystem)
	assert.Equal(t, 10001, frameworkerror.CodeParamInvalid)
	assert.Equal(t, 10002, frameworkerror.CodeAuthFailed)
	assert.Equal(t, 10003, frameworkerror.CodeConfigNotFound)
	assert.Equal(t, 10004, frameworkerror.CodeConfigInvalid)
	assert.Equal(t, 10005, frameworkerror.CodePolarisInit)
	assert.Equal(t, 10006, frameworkerror.CodePolarisGetConfig)
	assert.Equal(t, 10010, frameworkerror.CodeRPCUnavailable)
	assert.Equal(t, 10011, frameworkerror.CodeRPCTimeout)
	assert.Equal(t, 10012, frameworkerror.CodeRPCDecodeError)
	assert.Equal(t, 10013, frameworkerror.CodeRPCEncodeError)
	assert.Equal(t, 20601, frameworkerror.CodeObsInit)
	assert.Equal(t, 20602, frameworkerror.CodeObsExport)
	assert.Equal(t, 20603, frameworkerror.CodeObsTraceExport)
	assert.Equal(t, 20604, frameworkerror.CodeObsMetricExport)
	assert.Equal(t, 20605, frameworkerror.CodeObsRuntimeMetrics)
}

// TestPredefinedErrors 构造器 code + public 消息与原 go-common 定义逐值一致。
func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		code   int
		public string
	}{
		{"ErrSystem", frameworkerror.ErrSystem.Wrap(errors.New("x")), 10000, "system_error"},
		{"ErrParamInvalid", frameworkerror.ErrParamInvalid.Wrap(errors.New("x")), 10001, "param_invalid"},
		{"ErrAuthFailed", frameworkerror.ErrAuthFailed.Wrap(errors.New("x")), 10002, "auth_failed"},
		{"ErrConfigNotFound", frameworkerror.ErrConfigNotFound.Wrap(errors.New("x")), 10003, "config_not_found"},
		{"ErrConfigInvalid", frameworkerror.ErrConfigInvalid.Wrap(errors.New("x")), 10004, "config_invalid"},
		{"ErrPolarisInit", frameworkerror.ErrPolarisInit.Wrap(errors.New("x")), 10005, "polaris_init_error"},
		{"ErrPolarisGetConfig", frameworkerror.ErrPolarisGetConfig.Wrap(errors.New("x")), 10006, "polaris_get_config_error"},
		{"ErrRPCUnavailable", frameworkerror.ErrRPCUnavailable.Wrap(errors.New("x")), 10010, "rpc_unavailable"},
		{"ErrRPCTimeout", frameworkerror.ErrRPCTimeout.Wrap(errors.New("x")), 10011, "rpc_timeout"},
		{"ErrRPCDecodeError", frameworkerror.ErrRPCDecodeError.Wrap(errors.New("x")), 10012, "rpc_decode_error"},
		{"ErrRPCEncodeError", frameworkerror.ErrRPCEncodeError.Wrap(errors.New("x")), 10013, "rpc_encode_error"},
		{"ErrObsInit", frameworkerror.ErrObsInit.Wrap(errors.New("x")), 20601, "observability_init_error"},
		{"ErrObsExport", frameworkerror.ErrObsExport.Wrap(errors.New("x")), 20602, "observability_export_error"},
		{"ErrObsTraceExport", frameworkerror.ErrObsTraceExport.Wrap(errors.New("x")), 20603, "observability_trace_export_error"},
		{"ErrObsMetricExport", frameworkerror.ErrObsMetricExport.Wrap(errors.New("x")), 20604, "observability_metric_export_error"},
		{"ErrObsRuntimeMetrics", frameworkerror.ErrObsRuntimeMetrics.Wrap(errors.New("x")), 20605, "observability_runtime_metrics_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, public := goerror.Extract(tt.err)
			assert.Equal(t, tt.code, code)
			assert.Equal(t, tt.public, public)
		})
	}
}

// TestHTTPStatusRegistration init() 注册的细粒度映射经 goerror.HTTPStatus 生效，
// 全部 case 与原 go-common/error httpStatusByCode 逐值一致。
func TestHTTPStatusRegistration(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"param invalid → 400", frameworkerror.ErrParamInvalid.Wrap(errors.New("x")), 400},
		{"auth failed → 401", frameworkerror.ErrAuthFailed.Wrap(errors.New("x")), 401},
		{"system → 500", frameworkerror.ErrSystem.Wrap(errors.New("x")), 500},
		{"config not found → 500", frameworkerror.ErrConfigNotFound.Wrap(errors.New("x")), 500},
		{"config invalid → 500", frameworkerror.ErrConfigInvalid.Wrap(errors.New("x")), 500},
		{"rpc decode → 500", frameworkerror.ErrRPCDecodeError.Wrap(errors.New("x")), 500},
		{"rpc encode → 500", frameworkerror.ErrRPCEncodeError.Wrap(errors.New("x")), 500},
		{"rpc unavailable → 503", frameworkerror.ErrRPCUnavailable.Wrap(errors.New("x")), 503},
		{"rpc timeout → 504", frameworkerror.ErrRPCTimeout.Wrap(errors.New("x")), 504},
		{"polaris init → 503", frameworkerror.ErrPolarisInit.Wrap(errors.New("x")), 503},
		{"polaris get config → 503", frameworkerror.ErrPolarisGetConfig.Wrap(errors.New("x")), 503},
		{"obs init → 503", frameworkerror.ErrObsInit.Wrap(errors.New("x")), 503},
		{"obs export → 500", frameworkerror.ErrObsExport.Wrap(errors.New("x")), 500},
		{"obs trace export → 503", frameworkerror.ErrObsTraceExport.Wrap(errors.New("x")), 503},
		{"obs metric export → 503", frameworkerror.ErrObsMetricExport.Wrap(errors.New("x")), 503},
		{"obs runtime metrics → 503", frameworkerror.ErrObsRuntimeMetrics.Wrap(errors.New("x")), 503},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, goerror.HTTPStatus(tt.err))
		})
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-framework/error/ -count=1`
Expected: FAIL — 包不存在（build error: no Go files / cannot find package）

- [ ] **Step 3: 实现 `go-framework/error/error.go`**

```go
// Package frameworkerror 提供 go-framework 模块的错误码和预定义错误构造器。
//
// 错误码范围：
//   - 10000-10013: system/param/auth/config/Polaris/RPC
//   - 20601-20605: observability（obs 段由 framework 适配层 hertz/kitex 使用；
//     码值为 wire 契约，禁止改号）
//
// 细粒度 HTTP 状态码映射通过 init() 注册到 go-common/error 注册表。
package frameworkerror

import (
	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// Builder 是错误构建器类型别名。
type Builder = goerror.Builder

// 框架错误码 10000-10013。
const (
	// CodeSystem 系统内部错误（兜底）
	CodeSystem = 10000
	// CodeParamInvalid 参数无效
	CodeParamInvalid = 10001
	// CodeAuthFailed 鉴权失败
	CodeAuthFailed = 10002
	// CodeConfigNotFound 配置未找到
	CodeConfigNotFound = 10003
	// CodeConfigInvalid 配置无效
	CodeConfigInvalid = 10004
	// CodePolarisInit Polaris 初始化失败
	CodePolarisInit = 10005
	// CodePolarisGetConfig Polaris 获取配置文件失败
	CodePolarisGetConfig = 10006
	// CodeRPCUnavailable RPC 服务不可用
	CodeRPCUnavailable = 10010
	// CodeRPCTimeout RPC 超时
	CodeRPCTimeout = 10011
	// CodeRPCDecodeError RPC 解码错误
	CodeRPCDecodeError = 10012
	// CodeRPCEncodeError RPC 编码错误
	CodeRPCEncodeError = 10013
)

// Observability 错误码 20601-20605（obs 段，由 framework 适配层使用）。
const (
	// CodeObsInit Observability 初始化失败
	CodeObsInit = 20601
	// CodeObsExport Observability 导出失败
	CodeObsExport = 20602
	// CodeObsTraceExport Trace exporter 创建失败
	CodeObsTraceExport = 20603
	// CodeObsMetricExport Metric exporter 创建失败
	CodeObsMetricExport = 20604
	// CodeObsRuntimeMetrics Runtime metrics 启动失败
	CodeObsRuntimeMetrics = 20605
)

// 预定义框架错误构造器。
var (
	// ErrSystem 系统内部错误（兜底）
	ErrSystem = goerror.Code(CodeSystem).Public("system_error")
	// ErrParamInvalid 参数无效
	ErrParamInvalid = goerror.Code(CodeParamInvalid).Public("param_invalid")
	// ErrAuthFailed 鉴权失败
	ErrAuthFailed = goerror.Code(CodeAuthFailed).Public("auth_failed")
	// ErrConfigNotFound 配置未找到
	ErrConfigNotFound = goerror.Code(CodeConfigNotFound).Public("config_not_found")
	// ErrConfigInvalid 配置无效
	ErrConfigInvalid = goerror.Code(CodeConfigInvalid).Public("config_invalid")
	// ErrPolarisInit Polaris 初始化失败
	ErrPolarisInit = goerror.Code(CodePolarisInit).Public("polaris_init_error")
	// ErrPolarisGetConfig Polaris 获取配置文件失败
	ErrPolarisGetConfig = goerror.Code(CodePolarisGetConfig).Public("polaris_get_config_error")
	// ErrRPCUnavailable RPC 服务不可用
	ErrRPCUnavailable = goerror.Code(CodeRPCUnavailable).Public("rpc_unavailable")
	// ErrRPCTimeout RPC 超时
	ErrRPCTimeout = goerror.Code(CodeRPCTimeout).Public("rpc_timeout")
	// ErrRPCDecodeError RPC 解码错误
	ErrRPCDecodeError = goerror.Code(CodeRPCDecodeError).Public("rpc_decode_error")
	// ErrRPCEncodeError RPC 编码错误
	ErrRPCEncodeError = goerror.Code(CodeRPCEncodeError).Public("rpc_encode_error")
)

// 预定义 Observability 错误构造器。
var (
	// ErrObsInit Observability 初始化失败
	ErrObsInit = goerror.Code(CodeObsInit).Public("observability_init_error")
	// ErrObsExport Observability 导出失败
	ErrObsExport = goerror.Code(CodeObsExport).Public("observability_export_error")
	// ErrObsTraceExport Trace exporter 创建失败
	ErrObsTraceExport = goerror.Code(CodeObsTraceExport).Public("observability_trace_export_error")
	// ErrObsMetricExport Metric exporter 创建失败
	ErrObsMetricExport = goerror.Code(CodeObsMetricExport).Public("observability_metric_export_error")
	// ErrObsRuntimeMetrics Runtime metrics 启动失败
	ErrObsRuntimeMetrics = goerror.Code(CodeObsRuntimeMetrics).Public("observability_runtime_metrics_error")
)

// init 注册框架错误码的细粒度 HTTP 状态码映射。
// 映射与原 go-common/error httpStatusByCode 逐值一致。
func init() {
	goerror.RegisterHTTPStatuses(map[int]int{
		CodeParamInvalid:      400,
		CodeAuthFailed:        401,
		CodeSystem:            500,
		CodeConfigNotFound:    500,
		CodeConfigInvalid:     500,
		CodeRPCDecodeError:    500,
		CodeRPCEncodeError:    500,
		CodeObsExport:         500,
		CodeRPCUnavailable:    503,
		CodePolarisInit:       503,
		CodePolarisGetConfig:  503,
		CodeObsInit:           503,
		CodeObsTraceExport:    503,
		CodeObsMetricExport:   503,
		CodeObsRuntimeMetrics: 503,
		CodeRPCTimeout:        504,
	})
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-framework/error/ -count=1`
Expected: PASS（3 个测试函数全绿）

- [ ] **Step 5: 提交**

```bash
gofmt -w go-framework/error/
git add go-framework/error/
git commit -m "feat(go-framework): add frameworkerror package owning 10000/obs error codes (#27)"
```

---

### Task 3: go-middleware clickhouse + tls 包内错误码（TDD）

**Files:**
- Create: `go-middleware/clickhouse/errors.go`
- Create: `go-middleware/clickhouse/errors_test.go`
- Create: `go-middleware/tls/errors.go`
- Create: `go-middleware/tls/errors_test.go`

**Interfaces:**
- Consumes: `goerror.Code`、`goerror.RegisterHTTPStatuses`（Task 1）
- Produces: `clickhouse.CodeConnect/CodeQuery/CodeParseDSN`（20401–20403）+ `Err*`；`tls.CodeConnect/CodeSend/CodeInvalidConfig/CodeProducerInit`（20501–20504）+ `Err*`。Task 4 的调用方迁移依赖这些符号。

- [ ] **Step 1: 写失败测试 `go-middleware/clickhouse/errors_test.go`**

```go
package clickhouse_test

import (
	"errors"
	"testing"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/byx-darwin/go-tools/go-middleware/clickhouse"
	"github.com/stretchr/testify/assert"
)

// TestCodeValues 码值是 wire 契约，逐值锁定。
func TestCodeValues(t *testing.T) {
	assert.Equal(t, 20401, clickhouse.CodeConnect)
	assert.Equal(t, 20402, clickhouse.CodeQuery)
	assert.Equal(t, 20403, clickhouse.CodeParseDSN)
}

// TestPredefinedErrors 构造器 code + public 消息与原 go-common 定义一致。
func TestPredefinedErrors(t *testing.T) {
	code, public := goerror.Extract(clickhouse.ErrParseDSN.Wrap(errors.New("x")))
	assert.Equal(t, 20403, code)
	assert.Equal(t, "ch_parse_dsn_error", public)

	code, public = goerror.Extract(clickhouse.ErrConnect.Wrap(errors.New("x")))
	assert.Equal(t, 20401, code)
	assert.Equal(t, "ch_connect_error", public)

	code, public = goerror.Extract(clickhouse.ErrQuery.Wrap(errors.New("x")))
	assert.Equal(t, 20402, code)
	assert.Equal(t, "ch_query_error", public)
}

// TestHTTPStatusRegistration init() 注册映射与原 httpStatusByCode 一致。
func TestHTTPStatusRegistration(t *testing.T) {
	assert.Equal(t, 503, goerror.HTTPStatus(clickhouse.ErrConnect.Wrap(errors.New("x"))))
	assert.Equal(t, 500, goerror.HTTPStatus(clickhouse.ErrQuery.Wrap(errors.New("x"))))
	assert.Equal(t, 503, goerror.HTTPStatus(clickhouse.ErrParseDSN.Wrap(errors.New("x"))))
}
```

- [ ] **Step 2: 写失败测试 `go-middleware/tls/errors_test.go`**

```go
package tls_test

import (
	"errors"
	"testing"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	gotls "github.com/byx-darwin/go-tools/go-middleware/tls"
	"github.com/stretchr/testify/assert"
)

// TestCodeValues 码值是 wire 契约，逐值锁定。
func TestCodeValues(t *testing.T) {
	assert.Equal(t, 20501, gotls.CodeConnect)
	assert.Equal(t, 20502, gotls.CodeSend)
	assert.Equal(t, 20503, gotls.CodeInvalidConfig)
	assert.Equal(t, 20504, gotls.CodeProducerInit)
}

// TestPredefinedErrors 构造器 code + public 消息与原 go-common 定义一致。
func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		code   int
		public string
	}{
		{"ErrConnect", gotls.ErrConnect.Wrap(errors.New("x")), 20501, "tls_connect_error"},
		{"ErrSend", gotls.ErrSend.Wrap(errors.New("x")), 20502, "tls_send_error"},
		{"ErrInvalidConfig", gotls.ErrInvalidConfig.Wrap(errors.New("x")), 20503, "tls_invalid_config_error"},
		{"ErrProducerInit", gotls.ErrProducerInit.Wrap(errors.New("x")), 20504, "tls_producer_init_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, public := goerror.Extract(tt.err)
			assert.Equal(t, tt.code, code)
			assert.Equal(t, tt.public, public)
		})
	}
}

// TestHTTPStatusRegistration init() 注册映射与原 httpStatusByCode 一致。
func TestHTTPStatusRegistration(t *testing.T) {
	assert.Equal(t, 503, goerror.HTTPStatus(gotls.ErrConnect.Wrap(errors.New("x"))))
	assert.Equal(t, 500, goerror.HTTPStatus(gotls.ErrSend.Wrap(errors.New("x"))))
	assert.Equal(t, 503, goerror.HTTPStatus(gotls.ErrInvalidConfig.Wrap(errors.New("x"))))
	assert.Equal(t, 503, goerror.HTTPStatus(gotls.ErrProducerInit.Wrap(errors.New("x"))))
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./go-middleware/clickhouse/ ./go-middleware/tls/ -count=1`
Expected: FAIL — `undefined: clickhouse.CodeConnect` / `undefined: gotls.CodeConnect`

- [ ] **Step 4: 实现 `go-middleware/clickhouse/errors.go`**

```go
package clickhouse

import (
	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// ClickHouse 错误码 20401-20403。
const (
	// CodeConnect ClickHouse 连接失败
	CodeConnect = 20401
	// CodeQuery ClickHouse 查询失败
	CodeQuery = 20402
	// CodeParseDSN ClickHouse DSN 解析失败
	CodeParseDSN = 20403
)

// 预定义 ClickHouse 错误构造器。
var (
	// ErrConnect ClickHouse 连接失败
	ErrConnect = goerror.Code(CodeConnect).Public("ch_connect_error")
	// ErrQuery ClickHouse 查询失败
	ErrQuery = goerror.Code(CodeQuery).Public("ch_query_error")
	// ErrParseDSN ClickHouse DSN 解析失败
	ErrParseDSN = goerror.Code(CodeParseDSN).Public("ch_parse_dsn_error")
)

// init 注册 ClickHouse 错误码的细粒度 HTTP 状态码映射。
// 映射与原 go-common/error httpStatusByCode 逐值一致。
func init() {
	goerror.RegisterHTTPStatuses(map[int]int{
		CodeConnect:  503,
		CodeQuery:    500,
		CodeParseDSN: 503,
	})
}
```

- [ ] **Step 5: 实现 `go-middleware/tls/errors.go`**

```go
package tls

import (
	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// TLS 错误码 20501-20504。
const (
	// CodeConnect TLS 连接失败
	CodeConnect = 20501
	// CodeSend TLS 发送失败
	CodeSend = 20502
	// CodeInvalidConfig TLS 配置无效
	CodeInvalidConfig = 20503
	// CodeProducerInit TLS Producer 初始化失败
	CodeProducerInit = 20504
)

// 预定义 TLS 错误构造器。
var (
	// ErrConnect TLS 连接失败
	ErrConnect = goerror.Code(CodeConnect).Public("tls_connect_error")
	// ErrSend TLS 发送失败
	ErrSend = goerror.Code(CodeSend).Public("tls_send_error")
	// ErrInvalidConfig TLS 配置无效
	ErrInvalidConfig = goerror.Code(CodeInvalidConfig).Public("tls_invalid_config_error")
	// ErrProducerInit TLS Producer 初始化失败
	ErrProducerInit = goerror.Code(CodeProducerInit).Public("tls_producer_init_error")
)

// init 注册 TLS 错误码的细粒度 HTTP 状态码映射。
// 映射与原 go-common/error httpStatusByCode 逐值一致。
func init() {
	goerror.RegisterHTTPStatuses(map[int]int{
		CodeConnect:       503,
		CodeSend:          500,
		CodeInvalidConfig: 503,
		CodeProducerInit:  503,
	})
}
```

- [ ] **Step 6: 运行测试确认通过**

Run: `go test ./go-middleware/clickhouse/ ./go-middleware/tls/ -count=1`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
gofmt -w go-middleware/clickhouse/ go-middleware/tls/
git add go-middleware/clickhouse/errors.go go-middleware/clickhouse/errors_test.go go-middleware/tls/errors.go go-middleware/tls/errors_test.go
git commit -m "feat(go-middleware): add package-local ClickHouse/TLS error codes (#27)"
```

---

### Task 4: go-middleware 调用方迁移

**Files:**
- Modify: `go-middleware/clickhouse/client.go:9,19`
- Modify: `go-middleware/tls/producer.go:25,62,67,72,149`
- Modify: `go-middleware/tls/shipper.go:26,50,62`

**Interfaces:**
- Consumes: `clickhouse.ErrParseDSN`、`tls.CodeInvalidConfig/CodeSend/CodeProducerInit`（Task 3）

- [ ] **Step 1: 改 `clickhouse/client.go`**

删除 import 块中的 `goerror "github.com/byx-darwin/go-tools/go-common/error"`（连同其所在的 import 分组空行），并将：

```go
		return nil, goerror.ErrCHParseDSN.Wrap(err)
```

改为：

```go
		return nil, ErrParseDSN.Wrap(err)
```

- [ ] **Step 2: 改 `tls/producer.go`**

删除 import 块中的 `goerror "github.com/byx-darwin/go-tools/go-common/error"`（保留 `oops` 直连 import），全文替换（3 处）：

```go
			Code(goerror.CodeTLSInvalidConfig).
```

→

```go
			Code(CodeInvalidConfig).
```

以及（1 处）：

```go
			Code(goerror.CodeTLSSend).
```

→

```go
			Code(CodeSend).
```

- [ ] **Step 3: 改 `tls/shipper.go`**

删除 import 块中的 `goerror "github.com/byx-darwin/go-tools/go-common/error"`，并替换：

```go
			Code(goerror.CodeTLSInvalidConfig).
```

→

```go
			Code(CodeInvalidConfig).
```

以及：

```go
			Code(goerror.CodeTLSProducerInit).
```

→

```go
			Code(CodeProducerInit).
```

- [ ] **Step 4: 构建与测试 go-middleware**

Run: `go build ./go-middleware/... && go test ./go-middleware/... -count=1`
Expected: PASS（clickhouse/tls 既有测试 + Task 3 新测试全绿）

- [ ] **Step 5: lint go-middleware**

Run: `golangci-lint run --timeout=5m ./go-middleware/...`
Expected: 无输出（0 issues）

- [ ] **Step 6: 提交**

```bash
git add go-middleware/clickhouse/client.go go-middleware/tls/producer.go go-middleware/tls/shipper.go
git commit -m "refactor(go-middleware): use package-local ClickHouse/TLS error codes (#27)"
```

---

### Task 5: go-framework 调用方迁移 + 注册保证

**Files:**
- Create: `go-framework/hertz/register.go`
- Modify: `go-framework/config/polaris.go`、`go-framework/hertz/middleware/auth.go`、`go-framework/hertz/observability/provider.go`、`go-framework/kitex/observability/provider.go`、`go-framework/kitex/option/option.go`、`go-framework/kitex/rpcerror/error.go`
- Modify tests: `go-framework/hertz/response_test.go`、`go-framework/hertz/response_integration_test.go`、`go-framework/kitex/rpcerror/error_test.go`

**Interfaces:**
- Consumes: `frameworkerror.*`（Task 2）
- Produces: 所有 go-framework 内部引用切换到 frameworkerror；hertz 包经 blank-import 保证注册表加载。

- [ ] **Step 1: 新建 `go-framework/hertz/register.go`**

```go
package hertz

// blank-import go-framework/error，确保框架错误码 → HTTP 状态码的
// 细粒度映射注册表在任何使用 hertz 包的应用中生效
// （即使应用未直接使用 frameworkerror 符号）。
import (
	_ "github.com/byx-darwin/go-tools/go-framework/error"
)
```

- [ ] **Step 2: 改 `config/polaris.go`**

将 import `goerror "github.com/byx-darwin/go-tools/go-common/error"` 替换为 `frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"`（polaris.go 仅使用两个 Err 符号，不再需要 goerror），并替换：

```go
		return nil, goerror.ErrPolarisInit.Wrap(err)
```

→

```go
		return nil, frameworkerror.ErrPolarisInit.Wrap(err)
```

```go
		return nil, goerror.ErrPolarisGetConfig.Wrap(err)
```

→

```go
		return nil, frameworkerror.ErrPolarisGetConfig.Wrap(err)
```

- [ ] **Step 3: 改 `hertz/middleware/auth.go`**

新增 import `frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"`（保留 `goerror` —— `goerror.In(...)` 是 go-common 机制）。全文替换 6 处：

```go
			Code(goerror.CodeParamInvalid).
```

→

```go
			Code(frameworkerror.CodeParamInvalid).
```

- [ ] **Step 4: 改 `hertz/observability/provider.go`**

将 import `goerror "github.com/byx-darwin/go-tools/go-common/error"` 替换为 `frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"`（该文件仅使用 3 个 ErrObs 符号），并替换：

```go
		return nil, goerror.ErrObsTraceExport.Wrap(err)
```

→

```go
		return nil, frameworkerror.ErrObsTraceExport.Wrap(err)
```

```go
			return nil, goerror.ErrObsMetricExport.Wrap(err)
```

→

```go
			return nil, frameworkerror.ErrObsMetricExport.Wrap(err)
```

```go
			return nil, goerror.ErrObsRuntimeMetrics.Wrap(err)
```

→

```go
			return nil, frameworkerror.ErrObsRuntimeMetrics.Wrap(err)
```

- [ ] **Step 5: 改 `kitex/observability/provider.go`**

与 Step 4 完全相同的三处替换（import 换成 frameworkerror；`ErrObsTraceExport`/`ErrObsMetricExport`/`ErrObsRuntimeMetrics` 加 `frameworkerror.` 前缀）。

- [ ] **Step 6: `observability/tracer.go` 两文件无需改动（验证）**

`hertz/observability/tracer.go` 与 `kitex/observability/tracer.go` 仅使用 `goerror.Extract` 与 `goerror.ProjectCodeMin`（均为 go-common 保留机制），不改。验证：

Run: `grep -n "goerror\." go-framework/hertz/observability/tracer.go go-framework/kitex/observability/tracer.go`
Expected: 仅 `goerror.Extract` 与 `goerror.ProjectCodeMin` 各一处。

- [ ] **Step 7: 改 `kitex/option/option.go`**

将 import `goerror "github.com/byx-darwin/go-tools/go-common/error"` 替换为 `frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"`（该文件仅使用 ErrConfigInvalid/ErrSystem），全文替换：`goerror.ErrConfigInvalid` → `frameworkerror.ErrConfigInvalid`（3 处）、`goerror.ErrSystem` → `frameworkerror.ErrSystem`（1 处）。

- [ ] **Step 8: 改 `kitex/rpcerror/error.go`**

新增 import `frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"`（保留 `goerror` —— `goerror.Extract` 是机制）。替换：

```go
	return code == goerror.CodeRPCTimeout
```

→

```go
	return code == frameworkerror.CodeRPCTimeout
```

并更新包注释（第 1-5 行）：

```go
// Package rpcerror 提供 Kitex RPC 框架的错误分类与适配。
//
// 核心错误处理（错误码、构造函数、Extract、预定义错误）已迁移至 go-common/error。
// 本包仅保留 Kitex 特定的分类逻辑和 BizStatus 适配器。
package rpcerror
```

→

```go
// Package rpcerror 提供 Kitex RPC 框架的错误分类与适配。
//
// 框架错误码与预定义错误位于 go-framework/error（frameworkerror）；
// 错误构造与提取机制位于 go-common/error。
// 本包仅保留 Kitex 特定的分类逻辑和 BizStatus 适配器。
package rpcerror
```

- [ ] **Step 9: 改 `hertz/response_test.go`**

新增 import `frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"`；若改完后 `goerror` 不再被使用则删除其 import。全文替换：`goerror.ErrParamInvalid` → `frameworkerror.ErrParamInvalid`、`goerror.CodeParamInvalid` → `frameworkerror.CodeParamInvalid`、`goerror.ErrAuthFailed` → `frameworkerror.ErrAuthFailed`、`goerror.CodeAuthFailed` → `frameworkerror.CodeAuthFailed`、`goerror.ErrRPCTimeout` → `frameworkerror.ErrRPCTimeout`、`goerror.CodeRPCTimeout` → `frameworkerror.CodeRPCTimeout`。

- [ ] **Step 10: 改 `hertz/response_integration_test.go`**

新增 import `frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"`；全文替换：`goerror.ErrParamInvalid` → `frameworkerror.ErrParamInvalid`（2 处）、`goerror.CodeParamInvalid` → `frameworkerror.CodeParamInvalid`（1 处）；若 `goerror` 不再被使用则删除其 import。

- [ ] **Step 11: 改 `kitex/rpcerror/error_test.go`**

新增 import `frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"`（保留 `goerror` —— `goerror.Code(...)` 机制仍用）。替换：

- `goerror.Code(goerror.CodeParamInvalid)` → `goerror.Code(frameworkerror.CodeParamInvalid)`（2 处：TestOopsStatusAdapter、TestIsTimeout 的 bizOther）
- `int32(goerror.CodeParamInvalid)` → `int32(frameworkerror.CodeParamInvalid)`
- `goerror.ErrRPCUnavailable` → `frameworkerror.ErrRPCUnavailable`（2 处：TestClassify、TestIsFrameworkError）
- `goerror.Code(goerror.CodeRPCTimeout)` → `goerror.Code(frameworkerror.CodeRPCTimeout)`

- [ ] **Step 12: 构建与测试 go-framework**

Run: `go build ./go-framework/... && go test ./go-framework/... -count=1`
Expected: PASS（含 response_test、response_integration_test、rpcerror 测试——HTTP 状态经注册表仍为 400/401/504 等）

- [ ] **Step 13: lint go-framework**

Run: `golangci-lint run --timeout=5m ./go-framework/...`
Expected: 无输出（0 issues）

- [ ] **Step 14: 提交**

```bash
git add go-framework/
git commit -m "refactor(go-framework): migrate callers to frameworkerror with registration guarantee (#27)"
```

---

### Task 6: go-common/error 瘦身（删符号 + 范围兜底 + 测试重整 + example 迁移）

**Files:**
- Modify: `go-common/error/error.go`（最终形态见 Step 1）
- Modify: `go-common/error/error_test.go`（最终形态见 Step 3）
- Modify: `go-common/error/httpstatus_test.go`（补充范围兜底测试）
- Modify: `example/handler/common_error.go`

**Interfaces:**
- Consumes: 所有 go-framework/go-middleware 调用方已完成迁移（Task 4/5），go-common 的迁移符号无仓内消费者。
- Produces: 瘦身后的 go-common/error（机制 + 边界 + 注册表 + 范围兜底），#28 将在此基础上修正 `ProjectCodeMin`。

- [ ] **Step 1: 全仓 grep 确认被删符号无残留消费者**

Run:

```bash
grep -rn "goerror\.\(ErrSystem\|ErrParamInvalid\|ErrAuthFailed\|ErrConfig\|ErrPolaris\|ErrRPC\|ErrRedis\|ErrKafka\|ErrDB\|ErrES\|ErrCH\|ErrTLS\|ErrObs\|ErrData\|ErrLogin\|ErrToken\|ErrPermission\|ErrRateLimit\|ErrQuota\|ErrIPBlocked\|ErrAccount\|ErrOrder\|ErrBalance\|ErrVerification\|ErrOperation\|CodeSystem\|CodeParam\|CodeAuth\|CodeConfig\|CodePolaris\|CodeRPC\|CodeRedis\|CodeKafka\|CodeDB\|CodeES\|CodeCH\|CodeTLS\|CodeObs\|CodeData\|CodeLogin\|CodeToken\|CodePermission\|CodeRateLimit\|CodeQuota\|CodeIPBlocked\|CodeAccount\|CodeOrder\|CodeBalance\|CodeVerification\|CodeOperation\)" \
  --include='*.go' . | grep -v 'go-common/error/' | grep -v 'go-framework/error/' | grep -v 'go-auth/error/'
```

Expected: 无输出（go-auth/error 自有同名符号在排除项内；若出现其他命中，先迁移其消费者再继续）。

- [ ] **Step 2: 将 `go-common/error/error.go` 替换为最终形态**

```go
// Package error 提供基于 oops 的统一错误处理机制。
//
// 本包是纯机制包，不持有任何模块的具体错误码：
//
//   - 构造/提取机制：Code、In、Extract、ExtractWithFallback、AsOopsError
//   - 码段边界常量：Framework/Middleware/Project 的 Min/Max
//   - HTTP 状态注册表：RegisterHTTPStatuses 供各属主模块在 init() 注册
//     细粒度映射；HTTPStatus 先查注册表，再走范围兜底
//     （业务码 ≥ ProjectCodeMin → 200；其余 >0 → 500；非 oops → 200）
//
// 具体错误码由各属主模块定义：
//
//	go-framework/error (frameworkerror): 10000-10013 + obs 20601-20605
//	go-auth/error      (autherror):      40000-40099
//	go-middleware/clickhouse:            20401-20403
//	go-middleware/tls:                   20501-20504
//
// 用法：
//
//	import goerror "github.com/byx-darwin/go-tools/go-common/error"
//
//	// 自定义错误码
//	err := goerror.Code(40001).Public("data_duplicate").Wrap(err)
//
//	// 提取
//	code, msg := goerror.Extract(err)
//	httpStatus := goerror.HTTPStatus(err)
package error

import (
	"errors"

	"github.com/samber/oops"
)

// ── 范围常量 ──

// 错误码范围边界常量。
const (
	FrameworkCodeMin  = 10000 // go-framework 最小错误码
	FrameworkCodeMax  = 10499 // go-framework 最大错误码
	MiddlewareCodeMin = 20000 // go-middleware 最小错误码
	MiddlewareCodeMax = 20699 // go-middleware 最大错误码
	ProjectCodeMin    = 40000 // 项目自定义最小错误码
	ProjectCodeMax    = 59999 // 项目自定义最大错误码
)

// ── 构造函数 ──

// Builder 是 oops 错误构建器类型别名。
type Builder = oops.OopsErrorBuilder

// Code 创建带错误码的 oops 构建器。
func Code(code any) Builder { return oops.Code(code) }

// In 创建带 domain 的 oops 构建器。
func In(domain string) Builder { return oops.In(domain) }

// ── 提取函数 ──

// Extract 从 error 中提取 oops 错误码和公开消息。
// 非 oops 错误返回 (0, "")。
func Extract(err error) (code int, public string) {
	if err == nil {
		return 0, ""
	}
	var oopsErr oops.OopsError
	if errors.As(err, &oopsErr) {
		if c, ok := oopsErr.Code().(int); ok {
			return c, oopsErr.Public()
		}
		return 0, oopsErr.Public()
	}
	return 0, ""
}

// ExtractWithFallback 从 error 中提取错误码，非 oops 错误使用 fallbackCode。
func ExtractWithFallback(err error, fallbackCode int) (code int, public string) {
	if err == nil {
		return 0, ""
	}
	code, public = Extract(err)
	if code == 0 {
		return fallbackCode, err.Error()
	}
	return
}

// AsOopsError 将 error 转换为 oops.OopsError。
func AsOopsError(err error) (oops.OopsError, bool) {
	var oopsErr oops.OopsError
	if errors.As(err, &oopsErr) {
		return oopsErr, true
	}
	return oops.OopsError{}, false
}

// ── HTTP 状态码映射 ──

// HTTPStatus 从 error 中提取错误码，映射为 HTTP 状态码。
// 优先级：各模块注册的细粒度映射 → 范围兜底。
func HTTPStatus(err error) int {
	code, _ := Extract(err)
	return httpStatusForCode(code)
}

// httpStatusForCode 按注册表 + 范围兜底映射错误码到 HTTP 状态码。
func httpStatusForCode(code int) int {
	if status, ok := lookupHTTPStatus(code); ok {
		return status
	}
	switch {
	case code >= ProjectCodeMin:
		return 200 // 业务错误（RPC 调用成功，HTTP 200）
	case code > 0:
		return 500 // 未注册的框架/基础设施错误
	default:
		return 200 // 非 oops 错误 / 无错误码
	}
}

// IsClientError 判断错误码是否属于客户端错误（4xx）。
func IsClientError(code int) bool {
	s := httpStatusForCode(code)
	return s >= 400 && s < 500
}

// IsServerError 判断错误码是否属于服务端/基础设施错误（5xx）。
func IsServerError(code int) bool {
	return httpStatusForCode(code) >= 500
}

// IsBusinessErrorCode 判断错误码是否属于业务错误（200，RPC 成功）。
func IsBusinessErrorCode(code int) bool {
	return code >= ProjectCodeMin || (code < FrameworkCodeMin && code > 0)
}
```

注意：`httpstatus.go`（Task 1 创建）保留不动；原 `error.go` 中的所有具体码常量块、`Err*` 构造器块、`httpStatusByCode` switch 全部随整文件替换删除。

- [ ] **Step 3: 将 `go-common/error/error_test.go` 替换为重整后的最终形态**

```go
package error

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// init 注册测试专用码 39911 → 404。
// 放在 init() 而非某个 Test 函数内，保证与测试文件/函数的执行顺序无关
// （Go 按文件名字母序执行：error_test.go 先于 httpstatus_test.go）。
func init() {
	RegisterHTTPStatuses(map[int]int{39911: 404})
}

// ── 错误码范围 ──

func TestCodeConstants(t *testing.T) {
	assert.Less(t, FrameworkCodeMax, MiddlewareCodeMin)
	assert.Less(t, MiddlewareCodeMax, ProjectCodeMin)
}

// ── Code / Extract ──

func TestCode_Basic(t *testing.T) {
	original := errors.New("original error")
	err := Code(12345).Public("custom_error").Wrap(original)

	code, public := Extract(err)
	assert.Equal(t, 12345, code)
	assert.Equal(t, "custom_error", public)
}

func TestIn_Basic(t *testing.T) {
	original := errors.New("auth failed")
	err := In("auth").Code(12346).Public("token_expired").Wrap(original)

	code, public := Extract(err)
	assert.Equal(t, 12346, code)
	assert.Equal(t, "token_expired", public)
}

func TestExtract_NilError(t *testing.T) {
	code, public := Extract(nil)
	assert.Equal(t, 0, code)
	assert.Empty(t, public)
}

func TestExtract_NonOopsError(t *testing.T) {
	err := errors.New("plain error")
	code, public := Extract(err)
	assert.Equal(t, 0, code)
	assert.Empty(t, public)
}

func TestExtractWithFallback_NonOops(t *testing.T) {
	err := errors.New("plain error")
	code, public := ExtractWithFallback(err, 99999)
	assert.Equal(t, 99999, code)
	assert.Equal(t, "plain error", public)
}

func TestExtractWithFallback_OopsError(t *testing.T) {
	err := Code(12345).Public("custom").Wrap(errors.New("inner"))
	code, public := ExtractWithFallback(err, 99999)
	assert.Equal(t, 12345, code)
	assert.Equal(t, "custom", public)
}

func TestAsOopsError(t *testing.T) {
	err := Code(10001).Public("test").Wrap(errors.New("inner"))

	oopsErr, ok := AsOopsError(err)
	assert.True(t, ok)
	assert.Equal(t, 10001, oopsErr.Code())
}

func TestAsOopsError_NonOops(t *testing.T) {
	err := errors.New("plain")

	_, ok := AsOopsError(err)
	assert.False(t, ok)
}

// ── HTTP 状态码映射（范围兜底；细粒度映射见各属主模块测试）──

func TestHTTPStatus_Fallback(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"business code → 200", Code(40001).Public("data_duplicate").Wrap(errors.New("dup")), 200},
		{"unregistered infra code → 500", Code(20999).Public("unregistered").Wrap(errors.New("x")), 500},
		{"plain error → 200", errors.New("plain"), 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, HTTPStatus(tt.err))
		})
	}
}

func TestIsClientError(t *testing.T) {
	// 39911 已由本文件 init() 注册为 404。
	assert.True(t, IsClientError(39911))
	assert.False(t, IsClientError(40001)) // 业务码 → 200
	assert.False(t, IsClientError(20999)) // 未注册 → 500
}

func TestIsServerError(t *testing.T) {
	assert.True(t, IsServerError(20999))  // 未注册 >0 → 500
	assert.False(t, IsServerError(40001)) // 业务码 → 200
	assert.False(t, IsServerError(0))     // 无码 → 200
}

func TestIsBusinessErrorCode(t *testing.T) {
	assert.True(t, IsBusinessErrorCode(40010))
	assert.True(t, IsBusinessErrorCode(40001))
	assert.False(t, IsBusinessErrorCode(10000))
	assert.False(t, IsBusinessErrorCode(20001))
}
```

- [ ] **Step 4: 清理 `go-common/error/httpstatus_test.go` 中与新 error_test.go 重复的断言**

删除 Task 1 创建的 `TestIsClientError_RegisteredCode`（39905）与 `TestIsServerError_RegisteredCode`（39906）两个测试——其语义已被新 `error_test.go` 的 `TestIsClientError`（39911，init 注册）与 `TestIsServerError`（范围兜底）覆盖。保留 `TestRegisterHTTPStatuses_Lookup`（39901）、`TestRegisterHTTPStatuses_RegistryPrecedence`（39903）、`TestRegisterHTTPStatuses_DuplicatePanics`（39902）——这三个是注册表机制本身的测试。

注意：`error_test.go` 的 `init()` 注册 39911，httpstatus_test.go 保留的测试使用 39901/39902/39903，码值互不重叠，不会触发重复注册 panic。

- [ ] **Step 5: 改 `example/handler/common_error.go`**

新增 import `frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"`（保留 `goerror` —— `Code`/`Extract`/`HTTPStatus`/`IsClientError`/`IsBusinessErrorCode` 均为保留机制），替换：

```go
	err1 := goerror.ErrParamInvalid.Wrap(fmt.Errorf("username is required"))
```

→

```go
	err1 := frameworkerror.ErrParamInvalid.Wrap(fmt.Errorf("username is required"))
```

- [ ] **Step 6: 运行全仓测试**

Run: `go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1`
Expected: PASS（go-common 测试不再引用已删符号；属主模块测试经注册表验证全部细粒度映射）

- [ ] **Step 7: 构建 example**

Run: `go build ./example/...`
Expected: 成功

- [ ] **Step 8: lint go-common**

Run: `golangci-lint run --timeout=5m ./go-common/...`
Expected: 无输出（0 issues）

- [ ] **Step 9: 提交**

```bash
gofmt -w go-common/error/ example/handler/common_error.go
git add go-common/error/ example/handler/common_error.go
git commit -m "refactor(go-common): slim error package to mechanism + band boundaries + HTTP registry (#27)"
```

---

### Task 7: 文档更新（归属模型 + D6）

**Files:**
- Modify: `specs/00_overview.md`
- Modify: `CLAUDE.md`
- Modify: `go-middleware/README.md`

- [ ] **Step 1: 改 `specs/00_overview.md` §三 go-common 表行**

替换：

```
| `error` | 统一错误码定义（10000-59999）+ HTTP 状态码映射 |
```

→

```
| `error` | 错误处理机制（oops Builder/Extract）+ 码段边界常量 + HTTP 状态注册表 |
```

- [ ] **Step 2: 改 `specs/00_overview.md` §三 go-framework 表**

在 go-framework 包表中新增一行（紧随现有 hertz/kitex 行之后）：

```
| `error` | 框架错误码与预定义构造器（10000-10013 + obs 20601-20605，包名 frameworkerror） |
```

- [ ] **Step 3: 重写 `specs/00_overview.md` §四 错误码体系**

将整个 §四 代码块替换为：

````markdown
## 四、错误码体系

错误码由各属主模块定义（D6）；`go-common/error` 只提供机制、码段边界与 HTTP 状态注册表。

```
go-framework/error (frameworkerror)  10000-10013  ── system/param/auth/config/Polaris/RPC
  10000 CodeSystem            → HTTP 500
  10001 CodeParamInvalid      → HTTP 400
  10002 CodeAuthFailed        → HTTP 401
  10003 CodeConfigNotFound    → HTTP 500
  10004 CodeConfigInvalid     → HTTP 500
  10005 CodePolarisInit       → HTTP 503
  10006 CodePolarisGetConfig  → HTTP 503
  10010 CodeRPCUnavailable    → HTTP 503
  10011 CodeRPCTimeout        → HTTP 504
  10012 CodeRPCDecodeError    → HTTP 500
  10013 CodeRPCEncodeError    → HTTP 500

go-framework/error (frameworkerror)  obs 段 20601-20605  ── observability（framework 适配层使用）
  20601-20605  Obs  → HTTP 503（export 失败 20602 → 500）

go-auth/error (autherror)  40001-40009  ── token/session/device/JWT → HTTP 200

go-middleware  20000-20699（码段边界，包内按需定义）
  clickhouse   20401-20403  → HTTP 503（query 失败 20402 → 500）
  tls          20501-20504  → HTTP 503（send 失败 20502 → 500）
  redis/kafka/db/es 预留分配（尚无定义；需要时在各包内定义并 init() 注册）

项目业务       40100-59999  ── HTTP 200（RPC 调用成功；库内无预定义，以下为推荐分配）
  40010-40012  数据（NotFound/Duplicate/Conflict）
  40110-40113  认证（LoginFailed/TokenExpired/TokenInvalid/PermissionDenied）
  40210-40212  限制（RateLimit/QuotaExceeded/IPBlocked）
  40310-40314  状态（AccountDisabled/OrderInvalid/BalanceInsufficient/VerificationFailed/OperationDenied）
```

HTTP 状态映射机制：各属主模块在 `init()` 中调用 `goerror.RegisterHTTPStatuses` 注册细粒度映射；`goerror.HTTPStatus` 先查注册表，再走范围兜底（业务码 → 200，其余已注册外 >0 → 500，非 oops → 200）。

详见 `go-common/error/`（机制）、`go-framework/error/`（框架码）、`go-auth/error/`（认证码）、`go-middleware/{clickhouse,tls}/errors.go`（中间件码）。
````

注：推荐分配表沿用原值（含 40010-40012 等），仅作项目参考；`ProjectCodeMin` 常量值 40000 → 40100 的修正在 #28 执行，届时同步本表。

- [ ] **Step 4: 改 `specs/00_overview.md` §五 决策表**

在决策表末尾追加一行：

```
| 错误码归属（D6） | 各模块拥有自己的错误码；`go-common/error` 只提供机制 + 码段边界 + HTTP 状态注册表 |
```

- [ ] **Step 5: 改 `CLAUDE.md` Key Decisions 表**

在 `| D5 | Old modules | ...` 行后追加：

```
| D6 | Error code ownership | **Owning modules** define their codes; `go-common/error` = mechanism + band boundaries + HTTP registry | ✅ active |
```

- [ ] **Step 6: 改 `CLAUDE.md` Error Code Ranges 段**

替换：

```
go-framework: 10000-10499  (system, param, auth, config, RPC middleware)
go-middleware: 20000-20699 (redis, kafka, db, es, clickhouse, observability)
go-auth:       40000-40099 (token, session, device auth errors)
Project custom: 40100-59999 (business modules, external dependencies)
```

→

```
go-framework: 10000-10499  (system, param, auth, config, RPC; defined in go-framework/error + obs 20601-20605)
go-middleware: 20000-20699 (clickhouse 20401-20403, tls 20501-20504 defined in-package; redis/kafka/db/es bands reserved)
go-auth:       40000-40099 (token, session, device auth errors; defined in go-auth/error)
Project custom: 40100-59999 (business modules, external dependencies; no library predefinitions)
```

- [ ] **Step 7: 改 `CLAUDE.md` Workspace Layout 两处**

替换 go-common 下：

```
  error/                   → Unified error handling (error codes + oops constructors)
```

→

```
  error/                   → Error mechanism (oops Builder/Extract + band boundaries + HTTP status registry)
```

在 go-framework 布局段（`hertz/` 行附近）新增：

```
  error/                   → Framework error codes (10000-10013 + obs 20601-20605, package frameworkerror)
```

- [ ] **Step 8: 改 `go-middleware/README.md` 包一览表**

删除过时行：

```
| 根包 | 错误码定义（20000-20699） |
```

并将 clickhouse / tls 两行更新为：

```
| `clickhouse` | ClickHouse 原生协议客户端（基于 `clickhouse-go/v2`；含包内错误码 20401-20403） |
| `tls` | 火山引擎日志服务（Producer + FileShipper；含包内错误码 20501-20504） |
```

- [ ] **Step 9: 提交**

```bash
git add specs/00_overview.md CLAUDE.md go-middleware/README.md
git commit -m "docs: error-code ownership model (D6) across specs/CLAUDE/README (#27)"
```

---

### Task 8: 最终全量验证

**Files:** 无新增（验证专用任务；若发现漂移则最小修复并单独提交）

- [ ] **Step 1: 全模块构建**

Run: `go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...`
Expected: 成功，无输出

- [ ] **Step 2: go vet**

Run: `go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...`
Expected: 成功，无输出

- [ ] **Step 3: gofmt 检查**

Run: `gofmt -l $(find . -name '*.go' -not -path '*/vendor/*' -not -path './.git/*')`
Expected: 无输出（所有文件格式化）

- [ ] **Step 4: golangci-lint 逐模块**

Run: `for m in go-common go-auth go-middleware go-framework; do golangci-lint run --timeout=5m ./$m/... || exit 1; done`
Expected: 四个模块均 0 issues

- [ ] **Step 5: 全量测试**

Run: `go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1`
Expected: 全部 ok

- [ ] **Step 6: 死码符号终检**

Run: `grep -rn "ErrRedisConnect\|ErrKafkaConnect\|ErrDBConnect\|ErrESConnect\|ErrDataNotFound\|ErrRateLimit\|CodeTokenInvalid = 40112\|CodeTokenExpired = 40111" --include='*.go' .`
Expected: 无输出（死码已彻底删除；go-auth 的 40001/40002 token 码不受影响）

- [ ] **Step 7: 若 Step 1-6 出现失败**

做最小修复，重跑失败项；修复以独立 commit 提交：`fix: <具体修复> (#27)`。全部通过后无需额外 commit。
