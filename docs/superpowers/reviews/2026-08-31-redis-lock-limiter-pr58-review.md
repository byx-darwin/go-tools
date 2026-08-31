# Code Review Report — PR #58 (Issue #56)

**分支**：`feat/56-redis-lock-limiter` → `main`
**PR**：https://github.com/byx-darwin/go-tools/pull/58
**日期**：2026-08-31

> ⚠️ 本报告为汇总性文档，非 GitHub 正式 review。PR 作者与本次交付执行账号同为 `byx-darwin`，按 `gf-review` skill 规则禁止自我审查提交正式批准/驳回结论，需人工审查者在 GitHub 上完成正式 approve。

## 交付摘要

在 `go-middleware/redis` 新增：
- `Mutex`（分布式锁）：单实例 `SET NX PX` + Lua 原子释放 + watchdog 自动续期
- `Limiter`（限流器）：令牌桶算法，Lua 原子实现，API 对齐 `golang.org/x/time/rate.Limiter`
- 错误码 `CodeLockAcquire=20103`、`CodeLockRelease=20104`、`CodeLimiterEval=20105`

## 审查过程

1. **10 个实现任务**，每个均为 TDD 循环 + 独立 task reviewer 审查，全部 Approved
2. **1 次 Task 8 修复轮**：`ttlMillis()` 在 `rate<=0` 时溢出导致 `PEXPIRE` 报错，已修复并通过 scoped re-review
3. **全分支最终 review**（opus 模型）：发现 5 个 Important + 9 个 Minor
4. **1 次全分支修复轮**：一次性处理全部 5 个 Important + 1 个建议合并前处理的 Minor（选项重命名），通过 scoped re-review 确认全部解决

## 已解决的 Important 问题

| # | 问题 | 修复 |
|---|------|------|
| 1 | `AllowN` 传入负数 `n` 可静默突破限流语义 | Go 侧 `n<=0` 直接放行 + Lua 侧 `hmset` 前 clamp `min(burst, tokens)` |
| 2 | `Limiter.Wait` 与 `Mutex.Lock` 对 ctx 取消的错误返回形态不一致 | `Wait` 改为 `ErrLimiterEval.Wrap(ctx.Err())`，与 `Lock` 一致，`errors.Is` 仍可穿透 |
| 3 | 锁丢失（watchdog 续期失败）对调用方完全不可观测 | 已在 godoc 中显式声明该局限（未新增 API，避免范围蔓延） |
| 4 | 设计文档承诺的 godoc 用法示例缺失 | 已为 `Mutex`/`Limiter` 补充用法示例 |
| 5 | `NewLimiter` 不校验负数 `r`/`burst`，且 `rate=0` 会因 1s TTL 兜底被静默重置为"每秒放行" | clamp 负数为 0；`rate<=0` 时 TTL 兜底改为 24h |

## 已解决的 Minor（合并前处理）

- `WithTTL`/`WithRetryInterval` 重命名为 `WithMutexTTL`/`WithMutexRetryInterval`，避免与未来 `LimiterOption`/`ClientOption` 命名空间冲突（发布前零成本，发布后是破坏性变更）

## 已记录但延后处理的 Minor（不阻塞合并）

1. Redis 传输错误与"锁竞争/未持有"共用 `CodeLockAcquire`/`CodeLockRelease`（→409），实际应为 503，与 `CodeConnect` 一致
2. `Unlock` 的 `res==0` 分支未清空 `m.token`（无害，token 全局唯一）
3. 重复 `TryLock`（在 `ErrLockRelease` 后）可能遗留一个有界存活的旧 watchdog goroutine
4. watchdog 续期调用使用 `context.Background()` 无超时，依赖 go-redis 默认 `ReadTimeout`
5. `AllowN` 的 `now` 由客户端传入，多节点时钟不同步会导致限流临时收紧（fail-closed 方向，非正确性问题）
6. `hmset` 已弃用，建议改用 `HSET`
7. `renewScript` 缺少 godoc（unexported，仅风格一致性问题）
8. 两处测试覆盖粒度问题：`TestLimiter_Refill_OverTime` 未单独断言 burst 上限；`TestMutex_Watchdog_StopsAfterUnlock` 无法区分真实实现与空实现

## 测试验证

```
go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...   ✅
go vet 同上                                                                      ✅
go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... \
  -count=1 -race                                                                ✅ 全部通过
gofmt -l                                                                        ✅ 无输出
golangci-lint v2                                                                ⚠️ 本地环境不可用（v1.64.8 vs 仓库要求 v2，既有环境问题），需 CI 验证
```

## 结论

**技术评估**：核心分布式原语实现正确（Lua 原子性、token 校验、Unlock/watchdog 时序、`crypto/rand` 使用），经过充分的多轮审查与修复，无 Critical 缺陷，所有 Important 问题均已解决。剩余 Minor 均为打磨性质，已记录供后续跟进。

**建议**：Ready to merge（待人工在 GitHub 上完成正式 approve，以及 CI 上 golangci-lint v2 的最终确认）。
