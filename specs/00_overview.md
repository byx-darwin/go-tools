# go-tools × ncgo 模板配置对齐方案 — 总览

> 日期：2026-06-23
> 状态：✅ 已完成 — Phase 1-5 全部实施完毕 | 最后更新：2026-06-23

## 一、背景

当前存在两个项目：

| 项目 | 定位 | 仓库 |
|------|------|------|
| **go-tools** | Go 工具库（config / middleware / framework 适配） | `gitee.com/byx_darwin/go-tools` |
| **ncgo** | AI-friendly 脚手架 CLI，生成 Hertz/Kitex 微服务项目 | `github.com/byx-darwin/ncgo` |

**核心问题**：ncgo 生成的微服务项目在 config、interceptor、response、rpcerror、redis、kafka 等方面**内嵌完整实现代码**（模板拷贝），而不是引用 go-tools 的库。这导致：

1. 每个生成的项目都有大量重复代码
2. go-tools 改进后，已生成的项目无法自动受益
3. 两套配置结构体（go-tools 的 `config/redis.Config` vs ncgo 生成的 `RateLimitRedisConfig`）定义相似字段但命名不一致
4. 错误处理、响应格式在不同项目间有微妙差异

## 二、目标

```text
ncgo 生成的项目
    ↓ import
go-framework     ← Hertz / Kitex 框架适配（含 access log 中间件）
go-middleware     ← 中间件客户端（Redis / Kafka / DB / ES / CH）
go-common        ← 通用工具 + 结构化日志库（零框架依赖）
```

生成项目的代码从"内嵌完整实现"精简为"几行 import + 薄适配层"。

## 三、方案文档索引

| 文件 | 内容 |
|------|------|
| [01_inventory_mapping.md](./01_inventory_mapping.md) | ncgo 模板生成内容 vs go-tools 已有能力的逐项对照 |
| [02_split_plan_summary.md](./02_split_plan_summary.md) | go-tools 拆分为三库的摘要（详见 `spces/SPLIT_PLAN.md`） |
| [03_template_alignment.md](./03_template_alignment.md) | ncgo 模板改造方案：哪些生成代码改为 import |
| [04_config_schema_alignment.md](./04_config_schema_alignment.md) | 配置结构体 & YAML schema 对齐方案（含 D2 时间单位、敏感配置处理、日志配置） |
| [05_implementation_phases.md](./05_implementation_phases.md) | 分阶段实施计划（含已确认决策 D1-D5、回滚计划） |

## 四、关键决策点（待确认）

1. **拆分优先还是模板改造优先？** — 建议先完成 go-tools 拆库（阶段 1-3），再改 ncgo 模板（阶段 4）
2. **配置结构体是否统一？** — go-tools 的 config struct 作为"事实标准"，ncgo 模板直接引用
3. **向后兼容** — 已经用旧模板生成的项目是否需要迁移工具？
4. **go-tools 发布为独立库还是保持在 go workspace？** — 拆分后每个库独立发版 vs workspace 内联合发版
5. **错误码体系** — go-tools 的 `rpc_error.ErrorType` vs ncgo 的 `oops` 风格错误码，需要对齐

## 五、收益预览

| 维度 | 当前 | 对齐后 |
|------|------|--------|
| 生成项目代码量 | ~2000+ 行基础设施代码 | ~200 行薄适配 + import |
| 配置结构体定义 | 各项目各自定义 | 统一从 go-middleware / go-framework 引入 |
| Redis/Kafka 客户端 | 每个项目一份实现 | go-middleware 统一提供 |
| 错误处理 | oops + 自定义 rpcerror | go-framework/kitex/rpcerror 统一提供 |

## 六、已确认关键决策（2026-06-23）

| 编号 | 决策 | 结论 |
|------|------|------|
| D1 | Kafka 库选择 | **kafka-go**（与 ncgo 已选型一致），go-tools 现有 sarama 标记 deprecated |
| D2 | 配置时间单位 | **time.Duration**（YAML 写 `30s` 等标准格式） |
| D3 | 错误库标准 | **oops 为主**，go-framework 提供 pkg/errors → oops 转换桥 |
| D4 | 三库发布策略 | **独立发版**（go-common 变化最少，go-framework 变化最多） |
| D5 | 旧 go-tools 模块 | **已删除**（config/hertz/kitex/middleware/tools 已移除，go.work 仅保留三新库） |

## 八、实施进度（2026-06-23 完成）

| 阶段 | 内容 | 包数 | 状态 |
|------|------|------|------|
| Phase 1 | go-common 拆分 | 9 | ✅ |
| Phase 2 | go-middleware 拆分 | 7 | ✅ (+tls) |
| Phase 3 | go-framework 拆分 | 12 | ✅ |
| P0 | go-common/log | 2 | ✅ |
| P1 | accesslog + observability + tls | 5 | ✅ |
| Phase 5 | 验证 + 文档 | — | ✅ |

### 最终结构

```
go-tools/
├── go.work              ← 仅三新库
├── go-common/           ← 9 包 (crypto/cache/log/netutil/timeutil/httpclient/captcha/auth/log-adapters)
├── go-middleware/       ← 7 包 (redis/kafka/db/es/clickhouse/tls + errors)
├── go-framework/        ← 12 包 (config×3/hertz×4/kitex×5)
└── specs/               ← 设计文档 + 迁移指南
```

### 验证结果

| 检查项 | 结果 |
|--------|------|
| 构建 | 28 包全部通过 |
| 测试 | 25 包全部通过 |
| genproto 冲突 | 0 |
| `//go:build ignore` | 0 |
| 旧 import 路径 | 0 |

## 七、错误码段总览

```
┌──────────┬──────────────┬──────────────────────────────────────────┐
│   码段   │   归属       │  说明                                    │
├──────────┼──────────────┼──────────────────────────────────────────┤
│ 10000-99 │ go-framework │ 系统级 (Internal/Timeout/Unavailable)   │
│ 10100-99 │ go-framework │ 参数校验 (InvalidParam/BindError)       │
│ 10200-99 │ go-framework │ 认证授权 (Unauthorized/Forbidden)       │
│ 10300-99 │ go-framework │ 配置/资源 (NotFound/ConfigMissing)      │
│ 10400-99 │ go-framework │ RPC 中间件 (CallerNotAllowed/Panic)     │
├──────────┼──────────────┼──────────────────────────────────────────┤
│ 20000-99 │ go-middleware│ Redis 连接/池/命令/Sentinel/锁          │
│ 20100-99 │ go-middleware│ Kafka 连接/Producer/Consumer/消息       │
│ 20200-99 │ go-middleware│ DB 连接/查询/事务/约束                  │
│ 20300-99 │ go-middleware│ Elasticsearch 连接/索引/文档/查询       │
│ 20400-99 │ go-middleware│ ClickHouse 连接/写入/查询/表操作        │
│ 20500-99 │ go-middleware│ 预留：缓存通用 / 其他中间件              │
│ 20600-99 │ go-middleware│ 可观测性：日志上报 (TLS) / 链路追踪      │
├──────────┼──────────────┼──────────────────────────────────────────┤
│ 40000-99 │ 项目自定义   │ 业务模块 (User/Order/Payment...)        │
│ 50000-99 │ 项目自定义   │ 外部依赖 (第三方服务调用失败)            │
└──────────┴──────────────┴──────────────────────────────────────────┘
```

详见各中间件包的 `errors.go` 及 [05_implementation_phases.md](./05_implementation_phases.md)。
| 响应格式 | 每个项目内嵌 response.go | go-framework/hertz 统一提供 |
| 升级维护 | 改模板 + 重新生成 | 改库版本 + go get -u |
