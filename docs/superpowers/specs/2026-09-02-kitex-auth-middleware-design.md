# kitex 侧鉴权中间件设计（Issue #94）

## 背景

`go-framework/hertz/middleware/` 已提供 JWT / Session / Device 三种鉴权中间件（Options 模式、统一错误路由），但 `go-framework/kitex/middleware/` 目前只有 `accesslog` + `compat` 类型适配层，RPC 调用链路的身份透传/校验完全缺位；kitex 侧也没有显式的 panic recovery 中间件封装。

来源：多角色框架评审（架构完整度视角），评估日期 2026-09-02（Issue #94）。

## 目标

在 `go-framework/kitex/middleware/auth/` 新增：

1. JWT / Session / Device 三种鉴权中间件，每种都提供 Server（校验）+ Client（注入身份）对称实现
2. 独立的 `Recovery()` Server 端 panic recovery 中间件

对齐 hertz 侧的能力矩阵，同时适配 kitex 的 client-server 双端架构与 RPC 场景下的身份透传需求。

## 非目标

- 不修改 hertz 侧现有中间件（包括已知的 Session 中间件错误路由不一致问题，本 Issue 不做无关重构）
- 不引入 `WithPanicHandler` 之类的自定义 panic 上报回调钩子
- 不改变 `middleware/accesslog.go` + `middleware/compat/` 现有模式（新中间件走独立目录、独立依赖模式）

## 现状调研摘要

- **Hertz 鉴权中间件**：`JWTAuth[T any](secret, opts...) app.HandlerFunc`（泛型 Claims，Options 模式，`WithRevocationChecker`/`WithVerifyOptions`）、`SessionAuth(store) app.HandlerFunc`、`DeviceAuth(store, extract) app.HandlerFunc`。JWT/Device 统一走 `writeAuthError` → `goerror.HTTPStatus(err)` → `c.AbortWithError`；Session 直接 `c.AbortWithStatus`（历史遗留不一致，本次不动）。
- **Kitex middleware 现状**：`accesslog.go` 定义自定义 `Endpoint`/`Middleware` 类型（不直接 import kitex，避免 genproto 冲突），配合 `middleware/compat/compat.go` 适配成 `endpoint.Middleware`；`observability/suite.go` 则直接依赖 `github.com/cloudwego/kitex/pkg/endpoint`，提供真正的 `ServerMiddleware`/`ClientMiddleware` 对，用 `metainfo.WithValue`/`GetAllValues` 做透传 —— 本设计跟随后者模式。
- **go-auth**：`jwt.Sign/Verify/Refresh`、`session.Store`、`device.Store`、`autherror.Err*`，均框架无关（不依赖 hertz/kitex context），可直接复用。
- **kitex metadata**：`github.com/bytedance/gopkg/cloud/metainfo` 已有成熟用法，区分 `WithValue`（仅下一跳）与 `WithPersistentValue`（随调用链自动持久透传到所有下游，无需业务代码手动转发）。
- **错误路由**：`go-framework/kitex/rpcerror.OopsStatusAdapter` 可将 oops 错误适配为 Kitex `BizStatusErrorIface`；`frameworkerror` 已预留 `CodeTokenMissing/Invalid/Expired`、`CodeAuthFailed` 等鉴权码。
- **Panic recovery**：kitex 内置 panic→`kerrors.ErrPanic` 转换（`rpcerror` 已识别），但仓库内无自定义 recovery middleware；hertz 侧 `Responder.Middleware()` 的 `defer recover()` + 结构化日志模式可参考。

## 架构设计

### 包结构

```text
go-framework/kitex/middleware/auth/
  jwt.go        # JWTAuthServer / JWTAuthClient
  session.go    # SessionAuthServer / SessionAuthClient
  device.go     # DeviceAuthServer / DeviceAuthClient
  recovery.go   # Recovery
  options.go    # 共享 Option 类型、metainfo key 常量
  *_test.go
```

直接依赖 `github.com/cloudwego/kitex/pkg/endpoint`（跟随 `observability/suite.go` 模式），不走 `accesslog.go` 的自定义类型 + compat 间接层——鉴权中间件天然需要读写 kitex context/metainfo，直接依赖更自然。

### 组件设计

- **`jwt.go`**
  - `JWTAuthServer[T any](secret []byte, opts ...JWTOption) endpoint.Middleware`：从 incoming metainfo 读取 token → `go-auth/jwt.Verify[T]` → 失败走 `OopsStatusAdapter`（`autherror.Err*`）；成功后 claims 存入 ctx（包内 `contextKey` 类型防冲突），raw token 保留供下游透传判断
  - `JWTAuthClient[T any](tokenProvider func(ctx context.Context) (string, bool), opts ...JWTOption) endpoint.Middleware`：优先复用 ctx 中已校验通过的 token（B→C 场景，避免重新签发）；否则调用 `tokenProvider` 获取 token，用 `metainfo.WithPersistentValue` 写入 outgoing metadata
  - Options：`WithRevocationChecker`、`WithVerifyOptions`（语义对齐 hertz 侧同名 Option）
- **`session.go`** / **`device.go`**：结构对称，分别复用 `go-auth/session.Store` / `go-auth/device.Store`；Server 端校验失败同样走 `OopsStatusAdapter`
- **`recovery.go`**
  - `Recovery(opts ...RecoveryOption) endpoint.Middleware`：`defer recover()` 捕获 panic → `log.L().WithCategory(log.CategoryRPC)` 结构化记录（含 stack）→ 转换为 `frameworkerror` 内部错误码 → `OopsStatusAdapter` 返回，避免进程崩溃；仅提供 Server 端实现，不提供 `WithPanicHandler` 之类回调钩子
- **`options.go`**：统一的 metainfo key 常量（如 `metaKeyJWTToken`），避免各文件重复定义

### 数据流

1. 上游（网关/hertz 入口服务）持有已校验的身份，用 kitex Client 调下游：`JWTAuthClient` 把 token 通过 `metainfo.WithPersistentValue` 写入 outgoing metadata
2. 下游 kitex Server 收到请求：`JWTAuthServer` 从 incoming metainfo 取 token → `go-auth/jwt.Verify` 校验 → 失败短路返回 BizStatusError；成功则 claims 存入 ctx
3. 若该服务继续调用更下游（B→C）：`JWTAuthClient` 检测 ctx 中已有校验通过的身份并直接复用；`WithPersistentValue` 保证无需业务代码手动转发即可透传到 C
4. `Recovery()` 挂在中间件链最外层，捕获任意 panic 转换为标准错误返回并记录日志

Session/Device 中间件遵循同样的 Server 校验 + Client 注入 + persistent 透传模式。

### 错误处理

- kitex 三种鉴权中间件（JWT/Session/Device）与 Recovery 统一通过 `oops` 构造错误，经 `rpcerror.OopsStatusAdapter` 转换为 Kitex BizStatusError 返回
- 错误码复用 `go-auth/error`（token 相关：`ErrTokenInvalid`/`ErrTokenRevoked`/`ErrDeviceKicked` 等）与 `go-framework/error`（`CodeTokenMissing`/`CodeAuthFailed` 等）
- hertz 侧 Session 中间件的历史不一致（裸 `AbortWithStatus`）本次不动，仅在 kitex 新中间件中采用统一方案

## 测试计划

- 每个中间件独立单元测试：用 `metainfo.WithValue`/`WithPersistentValue` 构造 ctx 模拟 incoming/outgoing 值，覆盖校验通过/失败路径、错误码映射
- Recovery 中间件测试：构造故意 panic 的 endpoint，断言 panic 不外泄、返回预期错误、日志被记录
- 尽量复用 hertz 侧同名中间件的测试结构，保持风格一致

## 范围确认（Q&A 摘要）

| 问题 | 决策 |
|------|------|
| 覆盖范围 | 一次性实现 JWT/Session/Device 全部三种 + Recovery |
| 挂载方向 | 三者都做 Server（校验）+ Client（注入）对称实现 |
| 错误路由 | kitex 侧统一走 oops/OopsStatusAdapter；hertz Session 遗留不动 |
| Recovery 设计 | 独立 Server 端中间件，结构化日志 + oops 错误码，无回调钩子 |
| 包结构/依赖模式 | 跟随 observability 模式，直接依赖 `endpoint.Middleware`，放在 `middleware/auth/` |
| 多跳透传语义 | 用 `metainfo.WithPersistentValue`，身份自动持久透传到所有下游 |
