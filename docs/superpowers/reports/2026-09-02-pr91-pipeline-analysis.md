# CI/CD 流水线分析报告 — PR #91

- **仓库**: byx-darwin/go-tools
- **分析对象**: PR #91「feat(hertz): JWTAuth 中间件支持透传 Verify 选项」(`feat/85-jwt-verify-issuer` → `main`)
- **分析时间**: 2026-09-02
- **分析工具**: `gf pipeline report/status/jobs`（只读）
- **PR 状态**: 已通过 CI，auto-merge 生效，已于 2026-09-02T07:17:39Z 合并入 `main`

## 1. PR #91 本次 CI 运行

| 项 | 值 |
|---|---|
| Run ID | 33602416498 |
| 触发时间 | 2026-09-02T07:12:21Z |
| 完成时间 | 2026-09-02T07:17:06Z |
| 总耗时 | ~285s（4m45s） |
| 结论 | ✅ success（全部 5 个 job 通过） |
| Run URL | https://github.com/byx-darwin/go-tools/actions/runs/33602416498 |

Job 明细：

| Job | 状态 | 耗时 |
|---|---|---|
| go.mod hygiene | success | 3s |
| go-auth | success | 40s |
| go-common | success | 95s |
| go-framework | success | 158s |
| go-middleware | success | 281s（全流水线最长，决定了总墙钟时间） |

分析过程中该 run 处于 `running` 状态（`go-framework`/`go-middleware` 未完成），轮询约 3 分钟后转为 `success`；随后 PR 立即被 auto-merge 合并，未见异常。**本次 PR 运行本身无发现（no findings）**。

## 2. 分支/历史上下文

### 2.1 成功率趋势

| 范围 | 总运行数 | 成功率 | 平均耗时 |
|---|---|---|---|
| `main`，近 7 天 | 43 | 76.7% | 170.7s |
| `main`，近 30 天 | 50 | 74.0% | 176.9s |
| `feat/85-jwt-verify-issuer`，全部历史 | 1（本次） | 100%（仅 1 次样本，report 聚合口径显示 0% 因样本太少不具代表性） | 285s |

**发现 1（⚠️ Medium）：`main` 分支成功率长期低于 80% 健康阈值**（7 天 76.7%、30 天 74.0%），未见明显回升趋势。虽然本次 PR #91 运行通过，但仓库整体流水线稳定性偏低，值得关注根因（见 2.2）。

### 2.2 失败模式

抽样检查了 `main` 最近的 4 次失败运行的 job 级别数据：

| Run ID | 时间 | 失败 job | 说明 |
|---|---|---|---|
| 33576217060 | 2026-09-02T00:39 | `govulncheck (go-framework)`, `govulncheck (go-middleware)` | 安全扫描 job 失败，构建/测试 job 全部通过 |
| 33496823343 | 2026-09-01T10:19 | `go-framework`, `go-middleware` | 主构建/测试 job 同时失败 |
| 33490082361 | 2026-09-01T09:01 | `go-framework`, `go-middleware` | 主构建/测试 job 同时失败 |

**发现 2（⚠️ Medium，疑似规律性故障而非单纯 flaky）：`go-framework` 与 `go-middleware` 两个 job 在最近 4 次失败中出现 3 次同时失败**（且每次都是这两个 job 联合失败，`go-common`/`go-auth`/`go.mod hygiene` 从未在这些 run 中失败）。根据 `CLAUDE.md` 的依赖拓扑，`go-framework` 与 `go-middleware` 是兄弟模块，均依赖 `go-auth`+`go-common`——两者稳定同时失败但互相无依赖，指向两种可能：
  1. 共享的上游依赖（`go-auth`/`go-common`）在特定提交下引入了破坏性变更，两个下游模块均受影响，但因缓存/运行顺序等原因 `go-auth`/`go-common` 自身 job 未复现失败；
  2. CI 环境/依赖下载类的间歇性问题（如 module proxy 超时）同时命中了这两个耗时较长的 job。

  建议：抓取这 2-3 次失败 run 的 `go-framework`/`go-middleware` job 日志（`gf pipeline logs --pipeline-id <ID>`）确认具体报错信息，判断是代码回归还是基础设施抖动。此项超出本次只读分析范围，未执行日志抓取（`jobs` 已提供，`logs` 未进一步展开，避免范围蔓延）。

### 2.3 耗时分布 / 瓶颈

**发现 3（🟡 Low-Medium，耗时上升趋势 + 稳定瓶颈 job）**：
- 近期（过去几小时）`main` 单次运行总耗时集中在 270s–305s 区间（如 06:23 run 287s、05:48 run 304s、05:17 run 271s、03:27 run 279s、03:11 run 286s），**明显高于 30 天平均值 176.9s**，说明近期（很可能是最近几次提交，包括依赖增多或测试增多）流水线整体在变慢，而非孤立现象。PR #91 本次 285s 属于这一新常态区间，并非异常值，但相对历史基线已上涨 ~60%。
- 在 PR #91 的 5 个 job 中，`go-middleware`（281s）与 `go-framework`（158s）明显慢于 `go-common`（95s）、`go-auth`（40s）、`go.mod hygiene`（3s），`go-middleware` 是决定流水线总时长的瓶颈 job（其余 job 均已在 100s 内完成）。这与 §2.2 中该 job 也是失败重灾区的观察一致，建议后续优先排查 `go-middleware` 模块（redis/kafka/db/es/clickhouse/tls 等中间件客户端测试较多，可能存在慢测试或外部依赖等待）。

## 3. 结论与优先级建议

| 优先级 | 发现 | 建议动作 |
|---|---|---|
| P1 | `go-framework`/`go-middleware` 在近期失败中反复共同失败（3/4 次） | 抓取失败 run 日志定位根因（代码回归 vs 依赖/网络抖动），必要时补充针对性重试或修复 |
| P2 | `main` 30 天成功率 74%（7 天 76.7%），低于 80% 健康线 | 结合上条根因排查后复核成功率是否回升；若仍偏低需扩大取样评估是否为持续性问题 |
| P3 | 近期单次流水线耗时（270–305s）较 30 天均值（177s）上涨约 60%，`go-middleware` 为稳定瓶颈 | 排查 `go-middleware` 测试套件是否可优化（缓存、并行化、减少外部依赖等待） |

**本次 PR #91 CI 运行本身**：通过，无阻塞性问题，auto-merge 正常生效，无需额外处理。以上发现均为仓库级 CI 健康趋势观察，供后续独立跟进。

---
*本报告为只读分析，未触发/重跑/取消任何流水线，未修改 CI 配置。*
