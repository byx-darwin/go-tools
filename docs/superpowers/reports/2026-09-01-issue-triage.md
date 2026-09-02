# Issue Triage Report — byx-darwin/go-tools

**Date**: 2026-09-01
**Scope**: All open Issues (`gf issue list --state open`)
**Context**: Phase 4 post-delivery step of gf-workflow run for Issue #74 (shipped as PR #84). Issue #74 still shows `open` at triage time (merge/close not yet reflected) but was triaged per the skill's normal idempotent process, per instruction to not special-case it.

## Summary

- Open Issues found: 9 (#74–#81, #83; no #82 — that's a merged PR)
- Already `triage:done`: 0
- Newly triaged: 9
- Duplicates found: 0
- `type:unknown` assigned: 0

## Priority-Ranked Table

| # | Priority | Type | Title | Existing label |
|---|----------|------|-------|-----------------|
| 🔴 83 | urgent | bug | chore(ci): 修复 go-auth/revocation 未打 tag 导致的 `go mod tidy check` 持续失败 + 核查分支保护规则 | bug |
| 🟠 81 | high | bug | chore(auth): 排查间接依赖 golang-jwt/jwt/v4 v4.0.0 的引入来源 | (none) |
| 🟠 78 | high | docs | docs(auth): Session/Device Store 安全实现要求未在 godoc 中显式化 | documentation |
| 🟠 76 | high | enhancement | feat(auth): JWT 密钥强度缺少最小长度校验 | enhancement |
| 🟠 75 | high | bug | fix(auth): WithIssuer 在 Verify 路径未生效，存在越权风险 | bug |
| 🟠 74 | high | enhancement | feat(auth): Refresh Token 缺少轮换与复用检测机制 | enhancement |
| 🟡 77 | medium | docs | docs(auth): doc.go 依赖关系图与 CLAUDE.md 的 DAG 描述矛盾 | documentation |
| 🟢 80 | low | docs | docs(auth): JWT Claims 反射仅支持一层嵌入 RegisteredClaims，需在文档声明限制 | documentation |
| 🟢 79 | low | enhancement | refactor(auth): Store 接口扩展需预留 optional interface pattern 避免未来 Breaking Change | enhancement |

## Priority Distribution

| Priority | Count | % |
|----------|-------|---|
| urgent | 1 | 11.1% |
| high | 5 | 55.6% |
| medium | 1 | 11.1% |
| low | 2 | 22.2% |

## Type Distribution

| Type | Count | % |
|------|-------|---|
| bug | 3 | 33.3% |
| docs | 3 | 33.3% |
| enhancement | 3 | 33.3% |

## Rationale Notes

- **#83 (urgent)**: Issue body explicitly marks both sub-problems P0; CI `go mod tidy check` failure blocks every future PR merge on go-middleware/go-framework, and branch protection may not be gating merges on required checks — repo-wide blocking issue with no workaround.
- **#81 (high)**: Possible transitively-vulnerable dependency (`golang-jwt/jwt/v4 v4.0.0`, referenced CVE lineage). Treated as security-investigation bug; not yet confirmed as exploitable in this codebase, so `high` rather than `urgent`.
- **#78 (high)**: Missing godoc security requirements (SessionID entropy, session-fixation regeneration, concurrency atomicity, TTL cleanup) for Store implementations — silent omission could propagate real vulnerabilities into every downstream implementation.
- **#76 (high)**: Missing minimum JWT secret length validation — weak HMAC secrets are brute-forceable, direct auth-bypass risk on the core signing path.
- **#75 (high)**: `WithIssuer` silently no-ops on the Verify path, contradicting its apparent contract; realistic misconfiguration in shared-secret multi-service setups could enable authorization bypass. Not marked urgent since exploitation requires a specific shared-secret deployment pattern (i.e., a viable workaround/avoidance exists: use per-service secrets).
- **#74 (high)**: Refresh tokens lack rotation/reuse detection; a leaked refresh token has no revocation path. Core auth-flow security gap; retained `enhancement` type per its pre-existing label. Still open at triage time — normal triage applied per instructions regardless of pending closure via PR #84.
- **#77 (medium)**: Documentation contradiction (linear-chain vs. DAG) could mislead future contributors or AI assistants into introducing a real circular dependency — moderate but not urgent, no immediate runtime impact.
- **#80 (low)**: Documented limitation gap in reflection-based Claims handling; workaround (direct one-level embedding) already exists, silent-skip risk is a doc clarity issue rather than a defect.
- **#79 (low)**: Forward-looking API-evolution convention (optional interface pattern) to avoid a *future* breaking change; no current break, no urgency.

## Labels Applied

Each Issue received one `type:*`, one `priority:*`, and `triage:done`, added alongside any pre-existing default GitHub labels (`bug` / `documentation` / `enhancement`), which were left untouched.

```
#83  + type:bug          + priority:urgent + triage:done
#81  + type:bug          + priority:high   + triage:done
#80  + type:docs         + priority:low    + triage:done
#79  + type:enhancement  + priority:low    + triage:done
#78  + type:docs         + priority:high   + triage:done
#77  + type:docs         + priority:medium + triage:done
#76  + type:enhancement  + priority:high   + triage:done
#75  + type:bug          + priority:high   + triage:done
#74  + type:enhancement  + priority:high   + triage:done
```

All 9 `gf issue add-label` calls returned `"success": true`.

## Duplicates

None identified.

## Follow-up / Anomalies

- Issue #74, the trigger for this Phase-4 run, is still `state: open` at triage time even though PR #84 ("Closes #74") is described as shipped. This is expected per the task's own note (GitHub closes on merge, and the merge event may not have propagated to this listing yet) — no action taken here; out of scope for this triage skill.
- Issue #83 (urgent) documents a live, repo-wide CI blocker (`go mod tidy check` failing on go-middleware/go-framework) plus a possible branch-protection gap — worth flagging for immediate attention outside this triage flow.
