# 错误处理统一实施计划 — 维度 1

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将所有 `fmt.Errorf` / `errors.New` 迁移到 `oops` + 错误码体系，使错误有统一的结构化格式、明确的错误码、可链式匹配。

**Architecture:** 在 `go-common/error/error.go` 中集中新增框架/中间件错误码（遵循已有模式），在 `go-auth/error/error.go` 中新增 JWT 错误码。然后逐文件将 `fmt.Errorf` 替换为 `oops` 构造器调用，保持 `errors.Is` 链兼容。

**Tech Stack:** Go 1.25, `github.com/samber/oops`, `github.com/stretchr/testify`

## Global Constraints

- 错误码范围：go-framework 10000-10499，go-middleware 20000-20699，go-auth 40000-40099
- 不迁移 `tracer.go` 和 `response.go` 中的 panic record（`fmt.Errorf("panic: ...")`)
- 不迁移测试代码中的 `fmt.Errorf`
- 保持现有 `errors.Is` / `errors.As` 链兼容
- 现有测试使用 `assert.ErrorContains` 检查子串，迁移后错误消息必须仍包含相同关键字

## 错误码分配（与已有码不冲突）

已有的 `go-common/error/error.go` 已定义：
- TLS: CodeTLSConnect=20501, CodeTLSSend=20502
- ClickHouse: CodeCHConnect=20401, CodeCHQuery=20402
- Observability: CodeObsInit=20601, CodeObsExport=20602
- Framework: CodeConfigNotFound=10003, CodeConfigInvalid=10004, CodeAuthFailed=10002, CodeParamInvalid=10001

新增码（接续已有编号）：
```
go-common/error:
  CodePolarisInit       = 10005  // Polaris 初始化失败
  CodePolarisGetConfig  = 10006  // Polaris 获取配置失败
  CodeCHParseDSN        = 20403  // ClickHouse DSN 解析失败
  CodeTLSInvalidConfig  = 20503  // TLS 配置无效
  CodeTLSProducerInit   = 20504  // TLS Producer 初始化失败
  CodeObsTraceExport    = 20603  // Trace exporter 创建失败
  CodeObsMetricExport   = 20604  // Metric exporter 创建失败
  CodeObsRuntimeMetrics = 20605  // Runtime metrics 启动失败

go-auth/error:
  CodeJWTSignFailed     = 40007  // JWT 签名失败
  CodeJWTVerifyFailed   = 40008  // JWT 验证失败
  CodeJWTRefreshFailed  = 40009  // JWT 刷新失败
```

---

### Task 1: 新增错误码到 go-common/error/error.go

**Files:**
- Modify: `go-common/error/error.go`
- Modify: `go-common/error/error_test.go`

**Interfaces:**
- Consumes: 已有错误码常量结构
- Produces: 新增 8 个错误码常量 + 8 个预定义错误构造器 + HTTP 状态映射

- [ ] **Step 1: 在 go-common/error/error.go 的 "go-framework 预定义错误码" 区域新增 Polaris 配置错误码**

在 `CodeRPCEncodeError = 10013` 后面添加：

```go
	// CodePolarisInit Polaris 初始化失败
	CodePolarisInit = 10005
	// CodePolarisGetConfig Polaris 获取配置文件失败
	CodePolarisGetConfig = 10006
```

- [ ] **Step 2: 在 "ClickHouse 错误码" 区域新增**

在 `CodeCHQuery = 20402` 后面添加：

```go
	CodeCHParseDSN = 20403 // ClickHouse DSN 解析失败
```

- [ ] **Step 3: 在 "TLS 错误码" 区域新增**

在 `CodeTLSSend = 20502` 后面添加：

```go
	CodeTLSInvalidConfig = 20503 // TLS 配置无效
	CodeTLSProducerInit  = 20504 // TLS Producer 初始化失败
```

- [ ] **Step 4: 在 "Observability 错误码" 区域新增**

在 `CodeObsExport = 20602` 后面添加：

```go
	CodeObsTraceExport    = 20603 // Trace exporter 创建失败
	CodeObsMetricExport   = 20604 // Metric exporter 创建失败
	CodeObsRuntimeMetrics = 20605 // Runtime metrics 启动失败
```

- [ ] **Step 5: 在对应的预定义错误构造器区域新增**

在 "go-framework 预定义错误" var 块中添加：

```go
	ErrPolarisInit      = Code(CodePolarisInit).Public("polaris_init_error")
	ErrPolarisGetConfig = Code(CodePolarisGetConfig).Public("polaris_get_config_error")
```

在 "ClickHouse 预定义错误" var 块中添加：

```go
	ErrCHParseDSN = Code(CodeCHParseDSN).Public("ch_parse_dsn_error")
```

在 "TLS 预定义错误" var 块中添加：

```go
	ErrTLSInvalidConfig = Code(CodeTLSInvalidConfig).Public("tls_invalid_config_error")
	ErrTLSProducerInit  = Code(CodeTLSProducerInit).Public("tls_producer_init_error")
```

在 "Observability 预定义错误" var 块中添加：

```go
	ErrObsTraceExport    = Code(CodeObsTraceExport).Public("observability_trace_export_error")
	ErrObsMetricExport   = Code(CodeObsMetricExport).Public("observability_metric_export_error")
	ErrObsRuntimeMetrics = Code(CodeObsRuntimeMetrics).Public("observability_runtime_metrics_error")
```

- [ ] **Step 6: 更新 HTTP 状态映射函数 `httpStatusByCode`**

在 `case CodeObsInit:` 的 503 分支中添加新码：

```go
	case CodeObsInit, CodeObsTraceExport, CodeObsMetricExport, CodeObsRuntimeMetrics:
		return 503
```

在 `case CodeTLSConnect:` 的 503 分支中添加：

```go
	case CodeTLSConnect, CodeTLSInvalidConfig, CodeTLSProducerInit:
		return 503
```

在 `case CodeCHConnect:` 的 503 分支中添加：

```go
	case CodeCHConnect, CodeCHParseDSN:
		return 503
```

在 500 分支中，将 `CodeObsExport` 替换为：

```go
	case CodeObsExport:
		return 500
```

（CodeObsTraceExport 等已在 503 分支中）

在 500 分支中，将 `CodeTLSSend` 保留：

```go
	case CodeTLSSend:
		return 500
```

在 500 分支中，将 `CodeCHQuery` 保留：

```go
	case CodeCHQuery:
		return 500
```

- [ ] **Step 7: 在 error_test.go 中新增测试验证新错误码**

在 `TestPredefinedErrors` 的 tests 表中添加：

```go
		{"ErrPolarisInit", ErrPolarisInit.Wrap(errors.New("init")), CodePolarisInit, "polaris_init_error"},
		{"ErrCHParseDSN", ErrCHParseDSN.Wrap(errors.New("dsn")), CodeCHParseDSN, "ch_parse_dsn_error"},
		{"ErrTLSInvalidConfig", ErrTLSInvalidConfig.Wrap(errors.New("cfg")), CodeTLSInvalidConfig, "tls_invalid_config_error"},
		{"ErrObsTraceExport", ErrObsTraceExport.Wrap(errors.New("exp")), CodeObsTraceExport, "observability_trace_export_error"},
```

- [ ] **Step 8: 运行测试验证**

```bash
go test ./go-common/error/... -count=1 -v
```

Expected: 所有测试 PASS

- [ ] **Step 9: Commit**

```bash
git add go-common/error/error.go go-common/error/error_test.go
git commit -m "feat(error): add error codes for Polaris, TLS, ClickHouse, Observability"
```

---

### Task 2: 新增 JWT 错误码到 go-auth/error/error.go

**Files:**
- Modify: `go-auth/error/error.go`
- Modify: `go-auth/error/error_test.go`

**Interfaces:**
- Consumes: go-common/error 基础设施
- Produces: 3 个 JWT 错误码 + 3 个预定义错误构造器

- [ ] **Step 1: 在 go-auth/error/error.go 的错误码常量区域新增**

在 `CodeSessionExpired = 40006` 后面添加：

```go
	CodeJWTSignFailed    = 40007 // JWT 签名失败
	CodeJWTVerifyFailed  = 40008 // JWT 验证失败
	CodeJWTRefreshFailed = 40009 // JWT 刷新失败
```

- [ ] **Step 2: 在预定义错误构造器区域新增**

在 `ErrSessionExpired` 后面添加：

```go
	ErrJWTSignFailed    = goerror.Code(CodeJWTSignFailed).Public("jwt_sign_failed")    // JWT 签名失败
	ErrJWTVerifyFailed  = goerror.Code(CodeJWTVerifyFailed).Public("jwt_verify_failed") // JWT 验证失败
	ErrJWTRefreshFailed = goerror.Code(CodeJWTRefreshFailed).Public("jwt_refresh_failed") // JWT 刷新失败
```

- [ ] **Step 3: 在 error_test.go 中添加测试**

```go
func TestJWTErrorCodes(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		code   int
		public string
	}{
		{"ErrJWTSignFailed", ErrJWTSignFailed.Wrap(errors.New("sign")), CodeJWTSignFailed, "jwt_sign_failed"},
		{"ErrJWTVerifyFailed", ErrJWTVerifyFailed.Wrap(errors.New("verify")), CodeJWTVerifyFailed, "jwt_verify_failed"},
		{"ErrJWTRefreshFailed", ErrJWTRefreshFailed.Wrap(errors.New("refresh")), CodeJWTRefreshFailed, "jwt_refresh_failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, public := goerror.Extract(tt.err)
			assert.Equal(t, tt.code, code)
			assert.Equal(t, tt.public, public)
		})
	}
}
```

- [ ] **Step 4: 运行测试验证**

```bash
go test ./go-auth/error/... -count=1 -v
```

Expected: 所有测试 PASS

- [ ] **Step 5: Commit**

```bash
git add go-auth/error/error.go go-auth/error/error_test.go
git commit -m "feat(auth): add JWT error codes (40007-40009)"
```

---

### Task 3: 迁移 go-auth/jwt/token.go

**Files:**
- Modify: `go-auth/jwt/token.go`

**Interfaces:**
- Consumes: `autherror.ErrJWTSignFailed`, `autherror.ErrJWTVerifyFailed`, `autherror.ErrJWTRefreshFailed`
- Produces: 所有 fmt.Errorf 替换为 oops 构造

- [ ] **Step 1: 更新 import 块**

替换：

```go
import (
	"errors"
	"fmt"
	"reflect"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"

	autherror "github.com/byx-darwin/go-tools/go-auth/error"
)
```

为：

```go
import (
	"errors"
	"reflect"
	"time"

	"github.com/samber/oops"
	gojwt "github.com/golang-jwt/jwt/v5"

	autherror "github.com/byx-darwin/go-tools/go-auth/error"
)
```

- [ ] **Step 2: 替换 Sign 函数中的第一个 fmt.Errorf**

替换：

```go
		return "", fmt.Errorf("jwt.Sign: claims type %T does not implement jwt.Claims", claims)
```

为：

```go
		return "", oops.With("jwt.Sign").
			Code(autherror.CodeJWTSignFailed).
			Errorf("claims type %T does not implement jwt.Claims", claims)
```

- [ ] **Step 3: 替换 Sign 函数中的第二个 fmt.Errorf**

替换：

```go
		return "", fmt.Errorf("jwt.Sign: failed to sign token: %w", err)
```

为：

```go
		return "", oops.With("jwt.Sign").
			Code(autherror.CodeJWTSignFailed).
			Wrap(err)
```

- [ ] **Step 4: 替换 Verify 函数中的 fmt.Errorf**

替换：

```go
		return nil, fmt.Errorf("jwt.Verify: claims type %T does not implement jwt.Claims", zero)
```

为：

```go
		return nil, oops.With("jwt.Verify").
			Code(autherror.CodeJWTVerifyFailed).
			Errorf("claims type %T does not implement jwt.Claims", zero)
```

- [ ] **Step 5: 替换 Verify 函数中的 Wrap(fmt.Errorf(...))**

替换：

```go
	return nil, autherror.ErrTokenInvalid.Wrap(fmt.Errorf("jwt.Verify: invalid claims type"))
```

为：

```go
	return nil, oops.With("jwt.Verify").
		Code(autherror.CodeJWTVerifyFailed).
		Error("invalid claims type")
```

- [ ] **Step 6: 替换 Refresh 函数中的 fmt.Errorf**

替换：

```go
		return "", fmt.Errorf("jwt.Refresh: %w", err)
```

为：

```go
		return "", oops.With("jwt.Refresh").
			Code(autherror.CodeJWTRefreshFailed).
			Wrap(err)
```

- [ ] **Step 7: 运行测试验证**

```bash
go test ./go-auth/jwt/... -count=1 -v
```

Expected: 所有测试 PASS（`assert.ErrorContains` 检查的子串仍然存在）

- [ ] **Step 8: Commit**

```bash
git add go-auth/jwt/token.go
git commit -m "refactor(auth): migrate jwt/token.go from fmt.Errorf to oops"
```

---

### Task 4: 迁移 go-middleware/tls/ (producer.go + shipper.go)

**Files:**
- Modify: `go-middleware/tls/producer.go`
- Modify: `go-middleware/tls/shipper.go`

**Interfaces:**
- Consumes: `goerror.ErrTLSInvalidConfig`, `goerror.ErrTLSProducerInit`, `goerror.ErrTLSSend`
- Produces: 所有 fmt.Errorf 替换为 oops 构造

- [ ] **Step 1: 更新 producer.go import 块**

替换：

```go
import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/volcengine/volc-sdk-golang/service/tls"
)
```

为：

```go
import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/samber/oops"
	"github.com/volcengine/volc-sdk-golang/service/tls"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
)
```

注意：保留 `"fmt"` 因为 `fmt.Sprintf` 在后续代码中不使用，但 `parseJSONLine` 中的 `fmt.Sprint` 在 shipper.go 中使用。检查后如果 producer.go 中不再需要 fmt，则删除。

实际上 producer.go 中不再需要 `fmt`（替换后无 fmt 调用），删除 `"fmt"` import。

- [ ] **Step 2: 替换 producer.go 中的参数校验 fmt.Errorf**

替换：

```go
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("tls: endpoint is required")
	}
	if cfg.TopicID == "" {
		return nil, fmt.Errorf("tls: topic_id is required")
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("tls: region is required")
	}
```

为：

```go
	if cfg.Endpoint == "" {
		return nil, oops.With("tls.NewProducer").
			Code(goerror.CodeTLSInvalidConfig).
			Error("endpoint is required")
	}
	if cfg.TopicID == "" {
		return nil, oops.With("tls.NewProducer").
			Code(goerror.CodeTLSInvalidConfig).
			Error("topic_id is required")
	}
	if cfg.Region == "" {
		return nil, oops.With("tls.NewProducer").
			Code(goerror.CodeTLSInvalidConfig).
			Error("region is required")
	}
```

- [ ] **Step 3: 替换 producer.go 中的 flush fmt.Errorf**

替换：

```go
		return fmt.Errorf("tls: put logs: %w", err)
```

为：

```go
		return oops.With("tls.flush").
			Code(goerror.CodeTLSSend).
			Wrap(err)
```

- [ ] **Step 4: 更新 shipper.go import 块**

替换：

```go
import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)
```

为：

```go
import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/samber/oops"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
)
```

注意：保留 `"fmt"` 因为 `parseJSONLine` 中 `fmt.Sprint(v)` 仍在使用。

- [ ] **Step 5: 替换 shipper.go 中的 fmt.Errorf**

替换：

```go
	if cfg.FilePath == "" {
		return nil, fmt.Errorf("tls: file_path is required")
	}
```

为：

```go
	if cfg.FilePath == "" {
		return nil, oops.With("tls.NewFileShipper").
			Code(goerror.CodeTLSInvalidConfig).
			Error("file_path is required")
	}
```

替换：

```go
		return nil, fmt.Errorf("tls: create producer: %w", err)
```

为：

```go
		return nil, oops.With("tls.NewFileShipper").
			Code(goerror.CodeTLSProducerInit).
			Wrap(err)
```

- [ ] **Step 6: 运行测试验证**

```bash
go test ./go-middleware/tls/... -count=1 -v
```

Expected: 所有测试 PASS（`assert.ErrorContains(t, err, "endpoint")` 等子串匹配仍然成立，因为 oops 错误消息包含原始文本）

- [ ] **Step 7: Commit**

```bash
git add go-middleware/tls/producer.go go-middleware/tls/shipper.go
git commit -m "refactor(tls): migrate from fmt.Errorf to oops with error codes"
```

---

### Task 5: 迁移 go-middleware/clickhouse/client.go

**Files:**
- Modify: `go-middleware/clickhouse/client.go`

**Interfaces:**
- Consumes: `goerror.ErrCHParseDSN`
- Produces: fmt.Errorf 替换为 oops 构造

- [ ] **Step 1: 更新 import 块**

替换：

```go
import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)
```

为：

```go
import (
	"crypto/tls"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
)
```

- [ ] **Step 2: 替换 fmt.Errorf**

替换：

```go
			return nil, fmt.Errorf("clickhouse: parse DSN: %w", err)
```

为：

```go
			return nil, goerror.ErrCHParseDSN.Wrap(err)
```

- [ ] **Step 3: 运行测试验证**

```bash
go test ./go-middleware/clickhouse/... -count=1 -v
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add go-middleware/clickhouse/client.go
git commit -m "refactor(clickhouse): migrate from fmt.Errorf to oops"
```

---

### Task 6: 迁移 go-framework/config/polaris.go

**Files:**
- Modify: `go-framework/config/polaris.go`

**Interfaces:**
- Consumes: `goerror.ErrPolarisInit`, `goerror.ErrPolarisGetConfig`
- Produces: fmt.Errorf 替换为 oops 构造

- [ ] **Step 1: 更新 import 块**

替换：

```go
import (
	"fmt"

	"github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/pkg/model"
)
```

为：

```go
import (
	goerror "github.com/byx-darwin/go-tools/go-common/error"

	"github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/pkg/model"
)
```

- [ ] **Step 2: 替换 fmt.Errorf**

替换：

```go
		return nil, fmt.Errorf("polaris init failed: %w", err)
```

为：

```go
		return nil, goerror.ErrPolarisInit.Wrap(err)
```

替换：

```go
		return nil, fmt.Errorf("polaris get config file: %w", err)
```

为：

```go
		return nil, goerror.ErrPolarisGetConfig.Wrap(err)
```

- [ ] **Step 3: 运行构建验证**

```bash
go build ./go-framework/config/...
```

Expected: 编译通过

- [ ] **Step 4: Commit**

```bash
git add go-framework/config/polaris.go
git commit -m "refactor(config): migrate polaris.go from fmt.Errorf to oops"
```

---

### Task 7: 迁移 go-framework/hertz/observability/provider.go

**Files:**
- Modify: `go-framework/hertz/observability/provider.go`

**Interfaces:**
- Consumes: `goerror.ErrObsTraceExport`, `goerror.ErrObsMetricExport`, `goerror.ErrObsRuntimeMetrics`
- Produces: 3 处 fmt.Errorf 替换为 oops 构造

- [ ] **Step 1: 更新 import 块**

在 import 中添加：

```go
	goerror "github.com/byx-darwin/go-tools/go-common/error"
```

删除 `"fmt"`（如果替换后不再使用）。检查发现 `fmt.Sprintf` 在 `ServerMiddleware` 中仍使用（`fmt.Sprintf("%s %s", ...)`），保留 `"fmt"`。

- [ ] **Step 2: 替换 trace exporter 错误**

替换：

```go
		return nil, fmt.Errorf("observability: create trace exporter: %w", err)
```

为：

```go
		return nil, goerror.ErrObsTraceExport.Wrap(err)
```

- [ ] **Step 3: 替换 metric exporter 错误**

替换：

```go
			return nil, fmt.Errorf("observability: create metric exporter: %w", err)
```

为：

```go
			return nil, goerror.ErrObsMetricExport.Wrap(err)
```

- [ ] **Step 4: 替换 runtime metrics 错误**

替换：

```go
			if err := runtimemetrics.Start(runtimemetrics.WithMeterProvider(mp)); err != nil {
				return nil, fmt.Errorf("observability: start runtime metrics: %w", err)
			}
```

为：

```go
			if err := runtimemetrics.Start(runtimemetrics.WithMeterProvider(mp)); err != nil {
				return nil, goerror.ErrObsRuntimeMetrics.Wrap(err)
			}
```

- [ ] **Step 5: 运行构建验证**

```bash
go build ./go-framework/hertz/observability/...
```

Expected: 编译通过

- [ ] **Step 6: Commit**

```bash
git add go-framework/hertz/observability/provider.go
git commit -m "refactor(hertz/observability): migrate from fmt.Errorf to oops"
```

---

### Task 8: 迁移 go-framework/kitex/observability/provider.go

**Files:**
- Modify: `go-framework/kitex/observability/provider.go`

**Interfaces:**
- Consumes: `goerror.ErrObsTraceExport`, `goerror.ErrObsMetricExport`, `goerror.ErrObsRuntimeMetrics`（与 Task 7 相同）
- Produces: 3 处 fmt.Errorf 替换为 oops 构造

- [ ] **Step 1: 更新 import 块**

在 import 中添加：

```go
	goerror "github.com/byx-darwin/go-tools/go-common/error"
```

保留 `"fmt"`（`Middleware` 中 `fmt.Sprintf("%T", req)` 仍在使用）。

- [ ] **Step 2: 替换 trace exporter 错误**

替换：

```go
		return nil, fmt.Errorf("observability: create trace exporter: %w", err)
```

为：

```go
		return nil, goerror.ErrObsTraceExport.Wrap(err)
```

- [ ] **Step 3: 替换 metric exporter 错误**

替换：

```go
			return nil, fmt.Errorf("observability: create metric exporter: %w", err)
```

为：

```go
			return nil, goerror.ErrObsMetricExport.Wrap(err)
```

- [ ] **Step 4: 替换 runtime metrics 错误**

替换：

```go
			if err := runtimemetrics.Start(runtimemetrics.WithMeterProvider(mp)); err != nil {
				return nil, fmt.Errorf("observability: start runtime metrics: %w", err)
			}
```

为：

```go
			if err := runtimemetrics.Start(runtimemetrics.WithMeterProvider(mp)); err != nil {
				return nil, goerror.ErrObsRuntimeMetrics.Wrap(err)
			}
```

- [ ] **Step 5: 运行构建验证**

```bash
go build ./go-framework/kitex/observability/...
```

Expected: 编译通过

- [ ] **Step 6: Commit**

```bash
git add go-framework/kitex/observability/provider.go
git commit -m "refactor(kitex/observability): migrate from fmt.Errorf to oops"
```

---

### Task 9: 迁移 go-framework/hertz/middleware/auth.go

**Files:**
- Modify: `go-framework/hertz/middleware/auth.go`

**Interfaces:**
- Consumes: `goerror.CodeParamInvalid`, `goerror.ErrAuthFailed`
- Produces: `errors.New` 和 `fmt.Errorf` 替换为 oops 构造

- [ ] **Step 1: 更新 import 块**

替换：

```go
import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)
```

为：

```go
import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
)
```

注意：删除 `"errors"`（不再使用），保留 `"fmt"`（`fmt.Sprintf` 在签名比较中仍使用）。

- [ ] **Step 2: 替换 parseAuthorization 中的 errors.New 和 fmt.Errorf**

替换：

```go
	auth := string(request.Header.Peek("X-Signature"))
	if auth == "" {
		return "", "", 0, errors.New("authorization header is empty")
	}

	decoded, err := base64.StdEncoding.DecodeString(auth)
	if err != nil {
		return "", "", 0, fmt.Errorf("authorization base64 decode: %w", err)
	}

	kvs := make(map[string]string)
	for _, part := range strings.Split(string(decoded), "&") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			kvs[kv[0]] = kv[1]
		}
	}

	if ak = kvs["ak"]; ak == "" {
		return "", "", 0, errors.New("ak is empty")
	}
	if sign = kvs["sign"]; sign == "" {
		return "", "", 0, errors.New("sign is empty")
	}
	if tt, ok := kvs["t"]; !ok || tt == "" {
		return "", "", 0, errors.New("timestamp is empty")
	} else {
		t, _ = strconv.ParseInt(tt, 10, 64)
	}
```

为：

```go
	auth := string(request.Header.Peek("X-Signature"))
	if auth == "" {
		return "", "", 0, goerror.In("auth.parseAuthorization").
			Code(goerror.CodeParamInvalid).
			Error("authorization header is empty")
	}

	decoded, err := base64.StdEncoding.DecodeString(auth)
	if err != nil {
		return "", "", 0, goerror.In("auth.parseAuthorization").
			Code(goerror.CodeParamInvalid).
			Wrap(err)
	}

	kvs := make(map[string]string)
	for _, part := range strings.Split(string(decoded), "&") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			kvs[kv[0]] = kv[1]
		}
	}

	if ak = kvs["ak"]; ak == "" {
		return "", "", 0, goerror.In("auth.parseAuthorization").
			Code(goerror.CodeParamInvalid).
			Error("ak is empty")
	}
	if sign = kvs["sign"]; sign == "" {
		return "", "", 0, goerror.In("auth.parseAuthorization").
			Code(goerror.CodeParamInvalid).
			Error("sign is empty")
	}
	if tt, ok := kvs["t"]; !ok || tt == "" {
		return "", "", 0, goerror.In("auth.parseAuthorization").
			Code(goerror.CodeParamInvalid).
			Error("timestamp is empty")
	} else {
		t, _ = strconv.ParseInt(tt, 10, 64)
	}
```

- [ ] **Step 3: 运行测试验证**

```bash
go test ./go-framework/hertz/middleware/... -count=1 -v -run TestAuth
```

Expected: 所有 auth 相关测试 PASS

- [ ] **Step 4: Commit**

```bash
git add go-framework/hertz/middleware/auth.go
git commit -m "refactor(hertz/auth): migrate from errors.New/fmt.Errorf to oops"
```

---

### Task 10: 全量验证

**Files:** 无修改

- [ ] **Step 1: 构建所有模块**

```bash
go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...
```

Expected: 编译通过，无错误

- [ ] **Step 2: 运行 go vet**

```bash
go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...
```

Expected: 无警告

- [ ] **Step 3: 运行所有测试**

```bash
go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1
```

Expected: 所有测试 PASS

- [ ] **Step 4: 运行 golangci-lint（逐模块）**

```bash
for m in go-common go-auth go-middleware go-framework; do
  golangci-lint run --timeout=5m ./$m/... || exit 1
done
```

Expected: 无 lint 错误

- [ ] **Step 5: 验证 fmt.Errorf 残留**

```bash
grep -rn 'fmt\.Errorf\|errors\.New' --include='*.go' \
  $(find . -name '*.go' -not -path '*/vendor/*' -not -path '*_test.go' -not -path '*/.git/*') | \
  grep -v 'panic:' | grep -v '// ' | grep -v 'nolint'
```

Expected: 仅剩 `tracer.go` 中的 panic record（`fmt.Errorf("panic: ...")`），其他所有 fmt.Errorf/errors.New 已清除。

- [ ] **Step 6: Commit 最终状态（如有遗漏修复）**

```bash
git add -A
git commit -m "fix: address any remaining lint or test issues from error migration"
```

---

## 迁移完成检查清单

- [ ] go-auth/jwt/token.go: 6 处 fmt.Errorf → oops ✅
- [ ] go-middleware/tls/producer.go: 4 处 fmt.Errorf → oops ✅
- [ ] go-middleware/tls/shipper.go: 2 处 fmt.Errorf → oops ✅
- [ ] go-middleware/clickhouse/client.go: 1 处 fmt.Errorf → oops ✅
- [ ] go-framework/config/polaris.go: 2 处 fmt.Errorf → oops ✅
- [ ] go-framework/hertz/observability/provider.go: 3 处 fmt.Errorf → oops ✅
- [ ] go-framework/kitex/observability/provider.go: 3 处 fmt.Errorf → oops ✅
- [ ] go-framework/hertz/middleware/auth.go: 5 处 errors.New/fmt.Errorf → oops ✅
- [ ] tracer.go panic record: 保留不动 ✅
- [ ] response.go panic record: 保留不动 ✅
- [ ] 错误码总表更新 ✅
- [ ] HTTP 状态映射更新 ✅
- [ ] 全模块 build/vet/test/lint 通过 ✅
