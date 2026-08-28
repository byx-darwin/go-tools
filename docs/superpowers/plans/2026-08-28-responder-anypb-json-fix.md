# Responder Anypb JSON 展开修复实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 `Responder.writeResponse` 的 JSON 分支，使包含 `*anypb.Any` 的 `Response` 对象序列化时展开为业务字段而非 `{type_url, value}` 原始格式。

**Architecture:** 在 `writeResponse` 的 JSON 分支中增加 `proto.Message` 类型断言，优先使用 `protojson.Marshal` 序列化（能正确展开 `Any` 字段），失败或非 proto 对象时回退到 `ctx.JSON`。

**Tech Stack:** Go 1.25+, Hertz v0.10.5, google.golang.org/protobuf v1.36.11

**Spec:** `docs/superpowers/specs/2026-08-28-responder-anypb-json-fix-design.md`

## Global Constraints

- 模块路径：`github.com/byx-darwin/go-tools/go-framework`
- 测试框架：testify (assert + require) + hertz `ut.PerformRequest`
- Protobuf 依赖已存在：`google.golang.org/protobuf v1.36.11`
- 不改变 Protobuf 内容协商分支的行为
- 不改变非 proto 对象的 JSON 序列化行为
- 所有新代码必须通过 `golangci-lint` 静态检查

---

## Task 1: 添加 protojson 导入和 writeResponse 修复

**Files:**
- Modify: `go-framework/hertz/response.go:7-18` (import 块)
- Modify: `go-framework/hertz/response.go:230-237` (writeResponse 方法)

**Interfaces:**
- Consumes: `writeResponse(ctx *app.RequestContext, httpCode int, obj any)` — 现有签名不变
- Produces: 修改后的 `writeResponse`，对 `proto.Message` 类型使用 `protojson.Marshal`

- [ ] **Step 1: 添加 protojson 导入**

在 `go-framework/hertz/response.go` 的 import 块中，添加到 `"google.golang.org/protobuf/proto"` 之后：

```go
"google.golang.org/protobuf/encoding/protojson"
```

修改后的 import 块：

```go
import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)
```

- [ ] **Step 2: 修改 writeResponse 方法**

将 `go-framework/hertz/response.go:230-237` 的 `writeResponse` 方法替换为：

```go
// writeResponse 根据内容协商写入响应。
func (r *Responder) writeResponse(ctx *app.RequestContext, httpCode int, obj any) {
	switch negotiateContentType(ctx) {
	case consts.MIMEPROTOBUF:
		ctx.ProtoBuf(httpCode, obj)
	default:
		if protoMsg, ok := obj.(proto.Message); ok {
			b, err := protojson.MarshalOptions{
				EmitUnpopulated: false,
				UseProtoNames:   false,
			}.Marshal(protoMsg)
			if err == nil {
				ctx.Data(httpCode, consts.MIMEApplicationJSONUTF8, b)
				return
			}
		}
		ctx.JSON(httpCode, obj)
	}
}
```

- [ ] **Step 3: 验证编译**

Run: `cd go-framework && go build ./hertz/...`
Expected: 编译通过，无错误

- [ ] **Step 4: 提交**

```bash
cd go-framework
git add hertz/response.go
git commit -m "fix(hertz): writeResponse 使用 protojson 展开 Any 字段

Response.Data 为 *anypb.Any 时，JSON 序列化从 ctx.JSON 改为
protojson.Marshal，使前端收到展开后的业务字段而非 {type_url, value}。

Fixes byx-darwin/go-tools#52"
```

---

## Task 2: 添加单元测试验证 proto Any 展开

**Files:**
- Modify: `go-framework/hertz/response_test.go` (追加测试函数和辅助代码)

**Interfaces:**
- Consumes: `Response` struct（含 `Data *anypb.Any`），`Responder.Success()`
- Produces: 测试函数 `TestWriteResponse_ProtoMessage_ExpandsAny`

- [ ] **Step 1: 编写失败测试**

在 `go-framework/hertz/response_test.go` 文件末尾追加以下测试代码：

```go
// testProtoMsg 是用于测试的 protobuf 消息。
// 实际测试中使用 Response 自身（它包含 *anypb.Any 类型的 Data 字段）。

func TestWriteResponse_ProtoMessage_ExpandsAny(t *testing.T) {
	r := NewResponder()

	// 构造一个包含 *anypb.Any 的 Response（模拟 reply 方法的输出）
	anyData, err := anypb.New(
		&Response{Code: 200, Msg: "ok"},
	)
	require.NoError(t, err)

	resp := &Response{
		Code: 200,
		Msg:  "ok",
		Data: anyData,
	}

	// 构造 Hertz 测试上下文
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		r.writeResponse(c, http.StatusOK, resp)
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/test", nil)
	require.Equal(t, http.StatusOK, w.Code)

	// 反序列化为 map 检查结构
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))

	// 验证 code 和 msg
	assert.Equal(t, float64(200), result["code"])
	assert.Equal(t, "ok", result["msg"])

	// 验证 data 是展开后的业务字段，而非 {type_url, value}
	data, ok := result["data"].(map[string]any)
	require.True(t, ok, "data 应为展开后的对象，got: %v", result["data"])

	// 展开后应包含嵌套的 code/msg（因为内层也是 Response）
	assert.Contains(t, data, "code")
	assert.Contains(t, data, "msg")

	// 不应包含 anypb.Any 的原始字段
	assert.NotContains(t, data, "type_url")
	assert.NotContains(t, data, "@type")
}
```

同时需要在 import 块添加以下导入：

```go
"encoding/json"
"net/http"

"github.com/cloudwego/hertz/pkg/app"
"github.com/cloudwego/hertz/pkg/common/config"
"github.com/cloudwego/hertz/pkg/common/ut"
"github.com/cloudwego/hertz/pkg/route"
"google.golang.org/protobuf/types/known/anypb"
```

- [ ] **Step 2: 运行测试验证失败（RED）**

Run: `cd go-framework && go test ./hertz/ -run TestWriteResponse_ProtoMessage_ExpandsAny -v -count=1`
Expected: 测试编译失败（writeResponse 是未导出方法，需在同一包内测试，确认编译通过）

注意：`writeResponse` 是未导出方法，但测试文件 `package hertz` 在同一包内，可直接调用。

- [ ] **Step 3: 运行测试验证通过（GREEN）**

Run: `cd go-framework && go test ./hertz/ -run TestWriteResponse_ProtoMessage_ExpandsAny -v -count=1`
Expected: PASS

- [ ] **Step 4: 运行全部响应测试确保无回归**

Run: `cd go-framework && go test ./hertz/ -run "TestResponder|TestWriteResponse|TestRPCErrorRouter|TestNewResponder" -v -count=1`
Expected: 全部 PASS

- [ ] **Step 5: 提交**

```bash
cd go-framework
git add hertz/response_test.go
git commit -m "test(hertz): 验证 writeResponse 对 proto Any 字段的 JSON 展开

新增 TestWriteResponse_ProtoMessage_ExpandsAny 测试，
确认包含 *anypb.Any 的 Response 序列化后 data 字段为展开的业务对象。"
```

---

## Task 3: 运行完整验证套件

**Files:**
- 无（仅运行验证命令）

**Interfaces:**
- Consumes: Tasks 1-2 的改动
- Produces: 验证通过的工作代码

- [ ] **Step 1: 运行 go vet**

Run: `cd go-framework && go vet ./hertz/...`
Expected: 无问题

- [ ] **Step 2: 运行全部 hertz 包测试**

Run: `cd go-framework && go test ./hertz/... -count=1`
Expected: 全部 PASS

- [ ] **Step 3: 运行 golangci-lint**

Run: `cd go-framework && golangci-lint run --timeout=5m ./hertz/...`
Expected: 无 lint 错误

- [ ] **Step 4: 确认工作区构建**

Run: `cd go-framework && go build ./...`
Expected: 编译通过
