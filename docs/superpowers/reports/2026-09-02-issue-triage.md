# Issue Triage Report — byx-darwin/go-tools

**Date**: 2026-09-02
**Scope**: All open Issues (`gf issue list --state open`)
**Context**: Phase 4 post-delivery step of gf-workflow run for Issue #76 (shipped/merged). Ran full open-Issue triage per the skill's normal idempotent process.

## Summary

- Open Issues found: 8 (#76–#81, #83, #85)
- Already `triage:done`: 7 (#76–#81, #83 — carried over from the 2026-09-01 triage run, unchanged, idempotently skipped)
- Newly triaged: 1 (#85)
- Duplicates found: 0
- `type:unknown` assigned: 0

## Priority-Ranked Table

| # | Priority | Type | Title | Status |
|---|----------|------|-------|--------|
| 🔴 83 | urgent | bug | chore(ci): 修复 go-auth/revocation 未打 tag 导致的 `go mod tidy check` 持续失败 + 核查分支保护规则 | pre-existing |
| 🟠 85 | high | enhancement | feat(hertz): JWTAuth 中间件需支持透传 WithExpectedIssuer 校验 | newly triaged |
| 🟠 81 | high | bug | chore(auth): 排查间接依赖 golang-jwt/jwt/v4 v4.0.0 的引入来源 | pre-existing |
| 🟠 78 | high | docs | docs(auth): Session/Device Store 安全实现要求未在 godoc 中显式化 | pre-existing |
| 🟠 76 | high | enhancement | feat(auth): JWT 密钥强度缺少最小长度校验 | pre-existing |
| 🟡 77 | medium | docs | docs(auth): doc.go 依赖关系图与 CLAUDE.md 的 DAG 描述矛盾 | pre-existing |
| 🟢 80 | low | docs | docs(auth): JWT Claims 反射仅支持一层嵌入 RegisteredClaims，需在文档声明限制 | pre-existing |
| 🟢 79 | low | enhancement | refactor(auth): Store 接口扩展需预留 optional interface pattern 避免未来 Breaking Change | pre-existing |

## Priority Distribution

| Priority | Count | % |
|----------|-------|---|
| urgent | 1 | 12.5% |
| high | 4 | 50.0% |
| medium | 1 | 12.5% |
| low | 2 | 25.0% |

## Type Distribution

| Type | Count | % |
|------|-------|---|
| bug | 2 | 25.0% |
| docs | 3 | 37.5% |
| enhancement | 3 | 37.5% |

## Rationale Notes (newly triaged)

- **#85 (high, enhancement)**: Follow-up gap identified during Issue #75's final review — `go-auth/jwt.Verify`'s new `WithExpectedIssuer` option (#75) cannot be configured through the Hertz `JWTAuth[T]` middleware, which hardcodes a no-options `Verify` call. The issue body itself frames this explicitly as a "fix coverage gap" (spec omission), not an implementation defect, so `type:bug` was rejected in favor of `type:enhancement` (adding option pass-through capability), consistent with how the sibling security-hardening issue #76 was classified. Priority `high` (not `urgent`): `JWTAuth` is described as the primary Hertz consumer of go-auth and the most typical deployment shape for the issuer-validation threat model, but there is a workaround (bypass the middleware and call `Verify` directly with options), so it does not meet the `urgent` bar (no active exploit / no production outage / not fully blocked).

All other open Issues (#76–#81, #83) were already `type:*` + `priority:*` + `triage:done` labeled from the 2026-09-01 triage run and were skipped per the skill's idempotency rule; see `2026-09-01-issue-triage.md` for their original rationale.

## Labels Applied

```
#85  + type:enhancement + priority:high + triage:done   (newly applied)
#76–#81, #83  (unchanged, already triage:done)
```

`gf issue add-label 85 --label "type:enhancement" --label "priority:high" --label "triage:done"` returned `"success": true`.

## Duplicates

None identified.

## Follow-up / Anomalies

- The task context named "Issue #76, just merged" as the trigger for this Phase-4 run, but #76 still shows `state: open` at triage time (expected — GitHub close/merge propagation lag, same pattern observed for #74 in the prior triage run). No action taken; out of scope for this skill.
- Issue #83 (urgent, pre-existing) still documents a live, repo-wide CI blocker (`go mod tidy check` failing on go-middleware/go-framework) plus a possible branch-protection gap — unresolved since the 2026-09-01 report, worth flagging for attention outside this triage flow.
- No conflicts, ambiguous classifications, or Issues that resisted classification were encountered in this run.
