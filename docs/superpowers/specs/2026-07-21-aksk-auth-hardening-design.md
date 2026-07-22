# AK/SK 中间件安全加固设计（#21 / #22 / #23）

**日期:** 2026-07-21
**跟踪:** #34（Phase 1 组 B）· **里程碑:** 安全加固
**范围:** `go-framework/hertz/middleware/auth.go`（及其测试）。组内三 issue 同文件、同函数，须在同一 PR 协调落地。
**模式:** fast（安全修复）— 三个 Issue 已含完整 Context/Goal/验收标准，作为需求基线，本文档为设计决策与方案。

---

## 1. 背景与问题

`go-framework/hertz/middleware/auth.go` 现有 AK/SK 验签存在三处安全缺陷：

| Issue | 位置 | 缺陷 | 后果 |
|-------|------|------|------|
| #21 (⚡high) | `auth.go:47-51` | 验签失败且 `isDebug` 时回显 `server:<expected>`，即该请求的**正确签名** | 调试模式成为凭据预言机，可直接重放绕过认证 |
| #22 | `auth.go:107-112` | `signRequest` 仅签 `ak+path+"/"+t+"/"+ak`，**不含 method/query/body** | 捕获的 `GET ?amount=1` 签名可授权 `POST` 或改 query，重放+篡改 |
| #23 | `auth.go:47-48,100` | 中间件不强制时间戳新鲜度；`strconv.ParseInt` 吞错导致 `t=0` | 无限期重放；非法时间戳被接受 |

## 2. 设计决策

### 2.1 规范签名格式（#22 核心）

废弃旧的拼接格式，改为**换行分隔的规范请求串**，绑定完整请求语义：

```
stringToSign = ak + "\n" + method + "\n" + requestURI + "\n" + timestamp + "\n" + sha256hex(body)
signature    = hex( HMAC-SHA256( key = sk, msg = stringToSign ) )
```

字段定义：

| 字段 | 取值 | 说明 |
|------|------|------|
| `ak` | 请求头中的 AccessKey | 绑定签名到具体 ak（延续原设计的 ak 绑定意图，保留一次） |
| `method` | 大写 HTTP 方法，如 `GET`/`POST` | `string(c.Request.Method())` |
| `requestURI` | origin-form 请求目标 = path + `?` + query | `string(c.Request.RequestURI())`（原始请求行目标，含 query string） |
| `timestamp` | 十进制 int64 | `strconv.FormatInt(t, 10)` |
| `sha256hex(body)` | 原始 body 的 SHA-256 小写十六进制 | 无 body 时为**空输入的 SHA-256**（`e3b0c442…`），确定性覆盖“有 body”场景 |

- 选用换行分隔，消除旧拼接 `ak+path+"/"+t+"/"+ak` 的字段边界歧义。
- `requestURI` 使用原始请求目标（含 query），保证 client/server 计算同一字符串；客户端按其请求 URL 的 path?query 复现。
- body 始终参与哈希（空 body → 空输入哈希），统一且安全。

> **客户端说明（#22 验收“同步更新客户端签名生成逻辑”）：** 仓库内**无**独立客户端签名 helper——`signRequest`（服务端，未导出）是唯一规范实现，ncgo 生成的调用方按 godoc 复现。因此“客户端更新”= 同步更新 godoc 中描述的规范格式。**不**新增导出的共享签名 helper（避免扩大本次安全修复的 API 面）；如需要可作为独立 follow-up。

### 2.2 消除凭据预言机 + 常量时间比较（#21）

- 验签失败的响应**不再包含 `server:<expected>`**；调试与非调试模式统一返回通用“签名不匹配”信息（`debug` 模式下 403 + 通用文案，非 debug 401）。
- 签名比较由 `expected != sign` 改为 **`hmac.Equal([]byte(expected), []byte(sign))`**（常量时间，长度不符返回 false）。
- 不向客户端回显 expected；是否在服务端日志记录 expected 为可选项——本方案**不**记录原始 expected 到日志（避免密钥派生值落盘），仅返回通用文案，满足强制验收项。

### 2.3 时间戳新鲜度窗口（#23）

- `parseAuthorization` 不再吞 `strconv.ParseInt` 错误：解析失败 → 返回 error → 中间件拒绝（400）。
- 中间件**自身**强制新鲜度窗口，不依赖业务方 `GetSk`：
  - 默认窗口 **±5 分钟**；通过 **Functional Options** 可配置（遵循 `.claude/rules/options-pattern.md`）。
  - 校验时机：`parseAuthorization` 成功后、`GetSk` 之前。`|now - t| > window` → 拒绝（401）。在 GetSk 前拦截，兼具防重放与减轻无效请求对后端的压力。
- API 变更（向后兼容，variadic）：
  ```go
  func Auth(authFace AuthFace, opts ...Option) app.HandlerFunc
  type Option func(*authOptions)
  func WithTimestampWindow(window time.Duration) Option  // window<=0 → 保持默认 5min
  ```
  现有 `Auth(face)` 调用无需改动（仓库内当前无调用方）。

## 3. 受影响文件

| 文件 | 变更 |
|------|------|
| `go-framework/hertz/middleware/auth.go` | 重写 `signRequest`（新规范格式）、新增 `Option`/`WithTimestampWindow`/默认窗口、`Auth` 加 `opts`、`parseAuthorization` 处理 ParseInt 错误、移除 debug 回显、`hmac.Equal` 比较、新鲜度校验 |
| `go-framework/hertz/middleware/auth_test.go` | 更新 `signRequest` 单测；新增中间件级测试（hertz `ut`） |

**不改：** `example/`（未注册 AKSK Auth 路由）、`go-common/auth/ak.go`（`RefreshSK`/MD5 属 #24 组 C，越界）。

## 4. 测试计划

单元（`signRequest`）：
- 新格式确定性 / hex 长度 64
- method、query、body 任一不同 → 签名不同
- 空 body → 空输入 SHA-256 参与，确定性

中间件级（hertz `route.Engine` + `ut.PerformRequest`，镜像 `jwt_auth_test.go`）：
- 合法签名（覆盖 method+query+body）→ 200
- #21：签名错误 + debug → 响应体**不含** expected/server 签名（断言不泄漏）
- #21：常量时间比较路径不影响正确放行/拒绝
- #22：相同 ak/path/t 但改 method / 改 query / 改 body → 401/403（签名失效）
- #23：过期时间戳（now-10min）→ 拒绝；未来时间戳（now+10min）→ 拒绝；非法时间戳（`t=abc`）→ 400
- #23：`WithTimestampWindow` 自定义窗口生效

验证命令：`go test ./go-framework/... -count=1`

## 5. 兼容性与风险

- **破坏性（有意）：** 签名算法变更，旧客户端签名失效。旧方案本身不安全，破坏即修复目标。仓库内无客户端/示例依赖旧算法，影响面仅限仓库外调用方，通过 godoc 公告新格式。
- **API：** `Auth` 增加 variadic `opts`，源码级向后兼容。新增导出符号需 godoc（revive 要求）。
- **风险点：** `requestURI` 的 client/server 一致性——实现时用 hertz `ut` 验证带 query 请求的签名往返；若 `Header.RequestURI()` 在某些路径为空，回退到 `URI().Path()+"?"+QueryString()` 构造，并在测试中固化。
