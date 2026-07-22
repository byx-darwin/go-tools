# 错误码归属决策与迁移设计（Issue #27）

> **工作流**: wf-2026-07-21-001（full mode）· **跟踪**: #34（多角色审计 roadmap · Phase 2）
> **日期**: 2026-07-22 · **状态**: 设计已确认，待实施规划
> **范围**: 仅 #27（PR: `Closes #27, Refs #34`）；#28（码段重叠/去重）另行工作流

## 1. 背景与问题

架构分析发现：所有错误码 —— 包括 go-framework 的 10000 段与 go-middleware 的 20000 段 —— 都**物理定义在最底层的 `go-common/error/error.go`**（约 390 行）：

- go-framework 码：`CodeSystem=10000` … `CodeRPCEncodeError=10013`
- go-middleware 码：`CodeRedisConnect=20001` … `CodeObsRuntimeMetrics=20605`
- 项目业务码示例：40010–40314（数据/认证/风控/状态四组）

**后果**：任何上层模块新增错误码都必须修改 go-common → 强制 go-common 发版 → 上层模块无法独立版本化。这是对决策 D4（独立版本化）的最大威胁。

## 2. 事实发现（决策依据）

| # | 事实 | 证据 |
|---|------|------|
| 1 | **仓库无任何 git tag**，四个库均未发布，无外部消费者 | `git tag -l` 为空 |
| 2 | **Redis/Kafka/DB/ES 四个包不 import `go-common/error`**；20001–20399 码与构造器全部是死码 | 全包 grep，0 个文件引用 |
| 3 | **业务码 40010–40314 同样无消费者**（仅 example 用临时 `Code(40001)` 演示机制） | grep 仅命中定义处与 example |
| 4 | 活跃码只有三组：10000–10013（go-framework 使用）、20401–20403（go-middleware/clickhouse）、20501–20504（go-middleware/tls）；**Obs 20601–20605 的实际使用者是 go-framework 的 observability**（hertz + kitex），不是 go-middleware | grep 符号级消费清单 |
| 5 | **`go-auth/error` 已是目标样板**：自有 40001–40009 码 + 构造器，仅依赖 go-common 的 `Code()`/`Builder` 机制 | `go-auth/error/error.go` |
| 6 | 唯一的跨码段耦合点是 `httpStatusByCode`（go-common 内的 switch，把全段码映射为 HTTP 状态）；调用方仅 `go-framework/hertz/response.go` 与 `example/handler/common_error.go` | grep |
| 7 | go-framework 与 go-middleware 是**兄弟模块，互不依赖**（DAG 拓扑），故 HTTP 映射不能靠一方 import 另一方 | CLAUDE.md 依赖层级 |

## 3. 决策

**选定方案 (a)——错误码归属迁回属主模块**；`go-common/error` 瘦身为纯机制包（机制 + 码段边界 + HTTP 状态注册表）。

备选方案 (b)（保留集中化 + 文档化为"共享错误注册表"、修订 D4 表述）被否决。

**子决策：**

| 议题 | 结论 |
|------|------|
| 死码处理 | **删除**（符合 YAGNI；码段分配表保留在 specs，将来真正需要时在属主模块重新定义——届时不再需要 go-common 发版，正是本次重构目的） |
| Obs 段 20601–20605 | 定义迁入 `go-framework/error`，**码值不变**（码值是 wire 契约：BizStatus 码、specs 表；改号无收益徒增扰动） |
| HTTP 状态映射 | 注册表机制（见 §6），各模块 `init()` 注册细粒度映射，go-common 提供范围兜底 |
| 兼容层 | **不加** `Deprecated` 别名（无外部消费者；且别名会造成 go-common 反向依赖上层模块的循环） |

**选择 (a) 的理由：**

1. **窗口期**：破坏性变更成本在 #29 发布前为 0，发布后需兼容层或等 v2。
2. **实际规模小**：20000 段大半是死码，真正迁移的只有 23 个码（framework 11 + obs 5 + CH 3 + TLS 4），其余 32 个符号删除。
3. **有现成模板**：go-auth/error 已验证模式，照抄即可，无需新抽象。

## 4. 目标结构

```text
go-common/error/                    ← 纯机制包（~390 行 → ~150 行）
  ├─ 码段边界常量  FrameworkCodeMin/Max, MiddlewareCodeMin/Max, ProjectCodeMin/Max（值不变）
  ├─ 构造机制      Builder, Code(), In()
  ├─ 提取机制      Extract(), ExtractWithFallback(), AsOopsError()
  ├─ HTTP 注册表   RegisterHTTPStatuses() + HTTPStatus()（注册表 + 范围兜底）
  └─ 派生判定      IsClientError / IsServerError / IsBusinessErrorCode

go-framework/error/                 ← 新包，package frameworkerror（对齐 go-auth 的 autherror）
  ├─ 10000–10013  framework 码 + Err 构造器
  ├─ 20601–20605  Obs 码 + Err 构造器（godoc 注明"obs 段由 framework 适配层使用"）
  └─ init() → RegisterHTTPStatuses（细粒度映射原样迁入）

go-middleware/clickhouse/           ← 现有包，新增 errors.go
  └─ 20401–20403  CH 码 + Err 构造器 + init() 注册

go-middleware/tls/                  ← 现有包，新增 errors.go
  └─ 20501–20504  TLS 码 + Err 构造器 + init() 注册
```

## 5. 迁移清单

### 5.1 迁移（码值一律不变，wire 稳定）

| 符号 | 从 | 到 | 备注 |
|------|----|----|------|
| `CodeSystem`…`CodeRPCEncodeError`（10000–10013，11 个）+ 11 个 `Err*` 构造器 | go-common/error | go-framework/error | 含 `CodeConfigNotFound`（活跃段，随段迁移） |
| `CodeObsInit`…`CodeObsRuntimeMetrics`（20601–20605，5 个）+ 5 个 `Err*` | go-common/error | go-framework/error | 实际使用者是 hertz/kitex observability |
| `CodeCHConnect/Query/ParseDSN`（20401–20403，3 个）+ 3 个 `Err*` | go-common/error | go-middleware/clickhouse | 整段迁移，保留段完整性 |
| `CodeTLSConnect/Send/InvalidConfig/ProducerInit`（20501–20504，4 个）+ 4 个 `Err*` | go-common/error | go-middleware/tls | 同上 |

### 5.2 删除（全部死码，62 个导出符号 = 31 码常量 + 31 构造器）

- Redis 20001–20005、Kafka 20101–20105、DB 20201–20204、ES 20301–20302（码常量 + `Err*` 构造器）
- 业务码 40010–40314（数据/认证/风控/状态四组码常量 + 15 个 `Err*`）
  - 其中 `CodeTokenExpired=40111` / `CodeTokenInvalid=40112` 与 go-auth 的 `40002`/`40001` 同名不同值；删除后 token 错误域专属 `go-auth/error`，**提前化解 #28 的同名码歧义**

### 5.3 删除规则

有消费者的段（CH/TLS）整段迁回属主包；整段无人使用的（Redis/Kafka/DB/ES/业务）删除。码段分配表仍保留在 `specs/00_overview.md`（权威分配参考），将来真正需要时在属主模块重新定义。

### 5.4 调用方改动（同 PR 内完成）

| 文件 | 改动 |
|------|------|
| go-framework: `config/polaris.go`、`hertz/middleware/auth.go`、`hertz/observability/{provider,tracer}.go`、`kitex/observability/{provider,tracer}.go`、`kitex/option/option.go`、`kitex/rpcerror/error.go` | 改 import 与符号名（`goerror.ErrX` → `frameworkerror.ErrX`） |
| go-middleware: `clickhouse/client.go`、`tls/producer.go`、`tls/shipper.go` | 改用包内自有符号（`goerror.ErrCHParseDSN` → `ErrParseDSN` 等） |
| `example/handler/common_error.go` | `goerror.ErrParamInvalid` → `frameworkerror.ErrParamInvalid` |
| `go-framework/hertz/register.go`（新文件） | blank-import `go-framework/error`，保证只用 response 的应用也能加载注册表（sql-driver 式惯用法） |

### 5.5 符号命名

迁入属主包后去掉前缀冗余（`CodeCHParseDSN` → `clickhouse.CodeParseDSN`，`ErrTLSConnect` → `tls.ErrConnect`），符合 Go "包名即前缀" 惯例。新包名 `frameworkerror`（对齐 `autherror`）；调用方 alias 沿用就近风格，规划阶段统一确定。

## 6. HTTP 状态注册机制

```go
// go-common/error —— 注册表（RWMutex 保护；重复注册同一 code → panic，启动期暴露配置错误）
func RegisterHTTPStatuses(m map[int]int)   // 各模块 init() 调用

func HTTPStatus(err error) int {
    code, _ := Extract(err)
    if s, ok := lookup(code); ok { return s }   // ① 注册表细粒度映射
    switch {
    case code >= ProjectCodeMin: return 200     // ② 业务错误 → 200（RPC 成功）
    case code > 0:              return 500      // ③ 未注册的框架/基础设施错误兜底
    default:                    return 200      // ④ 非 oops / code 0（保持现行行为）
    }
}
```

**注册保证链：**

- `clickhouse` / `tls`：包自身 `init()`——import 谁，谁的映射就生效
- `frameworkerror`：自身 `init()`；`go-framework/hertz` 经 blank-import 兜底；kitex 侧 `rpcerror`/`option` 直接 import，天然保证
- 注册仅发生在 init 期（main 之前单线程），运行期只读

**行为兼容**：现存所有错误码的最终 HTTP 状态与今天逐值一致——现行 `httpStatusByCode` 的全部 case（400/401/500/503/504）原样搬进各模块注册表。`hertz/response.go` 与 `example` 的行为零变化。`IsClientError`/`IsServerError` 继续派生自 `HTTPStatus`，语义不变。

**注册表设计细节**：
- 存储：包级 `map[int]int` + `sync.RWMutex`
- 重复注册同一 code → panic（对齐 `database/sql.Register` 惯例，及早暴露复制粘贴错误）
- godoc 注明：`RegisterHTTPStatuses` 预期在包 `init()` 中调用，非并发安全的启动期 API

## 7. 文档更新

| 文件 | 改动 |
|------|------|
| `specs/00_overview.md` §四 错误码体系 | 重写为**归属模型**：码段分配表保留（权威分配参考），每段标注物理位置（`go-framework/error`、`go-auth/error`、`go-middleware/{clickhouse,tls}`）；业务 40xxx 注明"项目自定义域，表中为推荐分配，库内无预定义"；"详见"指针更新为新位置 |
| `specs/00_overview.md` §五 + `CLAUDE.md` 决策表 | 新增 **D6**：错误码归属——各模块拥有自己的错误码；`go-common/error` 只提供机制 + 码段边界 + HTTP 状态注册表 |
| `CLAUDE.md` Error Code Ranges 段 | 码段值不变，补注物理位置 |
| `go-common/error` 包 godoc | 重写：机制 + 边界 + 注册表定位，附注册用法示例 |
| 新文件 godoc | `go-framework/error`、`clickhouse/errors.go`、`tls/errors.go` 按 revive exported 规范写全注释（`// Name ...` 格式） |
| `go-framework/kitex/rpcerror/error.go:3` | "已迁移至 go-common/error" → 改指 `go-framework/error` |
| `go-middleware/README.md` | 如有错误码相关描述则同步（规划阶段核实） |

## 8. 测试策略

**go-common/error（新机制测试）：**
- 注册表命中：`RegisterHTTPStatuses` 后 `HTTPStatus` 返回注册值
- 重复注册同一 code → panic
- 兜底规则：`≥ ProjectCodeMin` → 200、未注册 `> 0` → 500、code 0 / 非 oops → 200
- 现有 `error_test.go` 中涉及被删符号/细粒度映射的用例：机制类保留，细粒度期望迁移到属主模块测试

**go-framework/error：**
- 码值不变断言（10000–10013、20601–20605 逐值）
- 端到端映射断言：把现行 `httpStatusByCode` 的**全部 case 逐值搬成测试**（400/401/500/503/504），确保行为零变化

**go-middleware：**
- `clickhouse`/`tls` 各码构造正确性 + 注册映射断言（Connect/ParseDSN→503、Query→500 等）

**集成：**
- `go-framework/hertz/response_integration_test.go`、`kitex/rpcerror/error_test.go` 换 import 后全绿

**执行方式**：Phase 3 逐任务 TDD（RED → GREEN → REFACTOR）。
**最终验证**：四模块 `go build` + `go vet` + `golangci-lint` v2 逐模块 + `go test -count=1` 全量。

## 9. 范围边界与非目标

**本 PR 做**：迁移 23 个活跃码（+ 23 个构造器，共 46 个符号）+ 删除 62 个死码符号 + HTTP 注册机制 + 调用方同步 + 文档（D6、specs、CLAUDE.md、godoc）。

**明确不做（留给后续）：**

| 事项 | 归属 |
|------|------|
| `ProjectCodeMin` 仍为 40000，原样搬迁；修为 40100 + 新增 `AuthCodeMin/Max` | #28（#27 搬常量、#28 改值，职责不混） |
| token 码去重的剩余部分（若有）与 specs 码表修订 | #28（同名歧义已被本次删除提前化解，PR 中注明） |
| 改任何码值 | 禁止（wire 稳定） |
| `Deprecated` 别名层 | 禁止（无外部消费者；会造成反向依赖） |
| 预建 Redis/Kafka/DB/ES 错误码 | 禁止（YAGNI） |
| 发布策略（replace/v0.0.0 剥离） | #29（roadmap 明确最后） |

## 10. 风险与缓解

| 风险 | 缓解 |
|------|------|
| ncgo 模板引用被移动/删除的符号 | 当前无 release/tag，ncgo 开发态经 workspace/replace 消费；PR 描述注明模板侧需同步；缓解措施为全仓库 grep 所有被删符号（含 `example/`） |
| 注册表全局状态 | 仅 init 期写入、运行期只读，RWMutex 保护；重复注册 panic 使配置错误在启动期暴露 |
| 只用 hertz/response 不用 middleware 的应用漏掉 framework 注册 | `go-framework/hertz` 包 blank-import `go-framework/error` 兜底 |
| 现有测试断言被删符号 | 规划阶段逐文件核对 `go-common/error/error_test.go`，细粒度期望迁移至属主模块 |

## 11. 验收对照（Issue #27 Acceptance Criteria）

- [x] 团队决策：方案 (a)，子决策见 §3（工作流 Gate 2→3 用户批准后生效）
- [ ] 按决策实施迁移（Phase 3）
- [ ] 迁移后各模块构建与测试通过（Phase 3 最终验证）
- [ ] 更新 `specs/00_overview.md` 错误码表（§7）
