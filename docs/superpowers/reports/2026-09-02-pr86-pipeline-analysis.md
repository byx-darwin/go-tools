# PR #86 CI/CD Pipeline Analysis

- Repo: byx-darwin/go-tools
- PR: #86 `fix(auth): WithIssuer 在 Verify 路径未生效，存在越权风险`（Closes #75）
- Branch: `feat/75-jwt-verify-issuer` → `main`
- Merged: 2026-09-02T01:23:45Z (merge commit `fdb7c04`)
- Analysis date: 2026-09-02
- Tooling: `gf pipeline report/status/jobs/logs` (read-only)

## 1. Success rate

| Scope | Runs | Success rate |
|---|---|---|
| `feat/75-jwt-verify-issuer` branch (30d) | 1 | 100% |
| `main` branch (30d, all workflows combined) | 42 | 66.7% (see §4 — unrelated to PR #86) |

PR #86 triggered exactly one pipeline run: **run 33578848183**, created 2026-09-02T01:18:21Z, completed 2026-09-02T01:23:14Z (4m53s), conclusion **success**.

## 2. Failure patterns

None for this PR — zero failed/retried jobs on the PR run.

## 3. Duration distribution (run 33578848183, CI workflow only)

| Job | Duration | Notes |
|---|---|---|
| go.mod hygiene | 8s | fastest, no Go toolchain setup needed |
| go-auth | 60s | |
| go-framework | 2m35s | |
| go-common | 1m33s | |
| go-middleware | **4m48s** | longest job, drives total wall time |

`go-middleware` is the bottleneck (matches its historical average — it has the most external client deps: redis/kafka/db/es/clickhouse/tls). Nothing anomalous versus its own baseline; not a new regression introduced by this PR (PR #86 only touches `go-auth` + `example/`).

## 4. CodeQL / govulncheck did NOT run — root cause: expected path filtering, not a gap

Confirmed via `.github/workflows/security.yml`:

```yaml
on:
  schedule:
    - cron: "0 0 * * 0"      # weekly, Sun 00:00 UTC
  push:
    branches: [main]
    paths: ["**/go.mod", "**/go.sum"]
  pull_request:
    branches: [main]
    paths: ["**/go.mod", "**/go.sum"]
  workflow_dispatch:
```

Both the `push` and `pull_request` triggers are **path-filtered to `go.mod`/`go.sum` changes only**. PR #86's diff (`git diff 6b8e21c..fdb7c04`) touched only:

```
example/handler/auth_jwt.go
go-auth/jwt/options.go
go-auth/jwt/token.go
go-auth/jwt/token_test.go
.claude/superpowers/{plans,specs}/...  (docs)
```

No `go.mod`/`go.sum` anywhere in the diff → **security.yml correctly did not fire**. This matches the CI workflow (`ci.yml`), which has no path filter and ran normally.

**Verdict: expected behavior, not a coverage gap.** PR #84/#85 triggered CodeQL/govulncheck because they were dependency-bump PRs (grpc/x-text CVE fixes) that did touch `go.mod`/`go.sum`.

Coverage is still maintained via:
- weekly cron (`0 0 * * 0`) scanning `main` regardless of what changed
- `push`-to-`main` trigger firing whenever a *merged* PR does touch go.mod/go.sum (as PR #84 did)
- `workflow_dispatch` for on-demand runs

Residual risk: a PR that changes Go **source** behavior in a way that newly reaches an already-known-vulnerable but previously-unreached symbol (via `govulncheck`'s call-graph reachability) would not be re-scanned until the next weekly cron, since it doesn't touch go.mod/go.sum. This is a standard, accepted trade-off of path-filtered security scanning (avoids redundant scans on every doc/logic PR) and not specific to PR #86 — flagged here only as a general observation, no action needed for this PR.

## 5. Context: transient govulncheck failures around the PR #84 merge window (unrelated to PR #86)

While pulling `main`'s 30-day history for comparison, two govulncheck runs failed shortly before PR #86 was opened:

- Run 33576217060 (2026-09-02T00:39:21Z, on `main` at commit `6b8e21c`, right after PR #84 merged): `govulncheck (go-framework)` failed on **GO-2026-6061** (grpc xDS/HTTP2 RBAC vuln, found `google.golang.org/grpc@v1.81.1`, fixed in `v1.82.1`); `govulncheck (go-middleware)` failed on **GO-2026-5970** (`golang.org/x/text` infinite-loop vuln, found `v0.38.0`, fixed in `v0.39.0`), reached via `tls/producer.go` init.
- By the very next paired run 9 minutes later (33576814547 / 33576814579, 2026-09-02T00:48Z), both `govulncheck` jobs were green again, and current `origin/main` go.mod files confirm the fixed versions are in place (`go-framework/go.mod`: grpc v1.82.1; `go-middleware/go.mod`: x/text v0.39.0).

This self-resolved before PR #86 was even opened (01:18Z) and PR #86's own run is fully green, so it did not affect PR #86. Included here only as a data point: it looks like a brief propagation lag between the go.work.sum dependency-bump commits landing on `main` (part of PR #84's merge sequence) rather than an actual unresolved vulnerability — worth a quick sanity check by whoever owns dependency bumps, but not an action item for PR #86.

## 6. Flaky tests

None observed for PR #86 — single run, no retries, no intermittent failures within the run.

## Summary

| Dimension | Finding |
|---|---|
| Success rate | 1/1 (100%) for the PR run |
| Failure patterns | None |
| Duration | 4m53s total, `go-middleware` job is the bottleneck (baseline behavior, not a regression) |
| CodeQL/govulncheck absence | **Expected** — `security.yml` path-filters on `**/go.mod`/`**/go.sum`; PR #86 changed neither. Weekly cron + push-to-main filter remain as safety net. |
| Flaky tests | None |

No action items for PR #86 itself. Optional/non-blocking: confirm the `go-framework` / `go-middleware` govulncheck blip around 00:39–00:48 UTC on 2026-09-02 was a one-off propagation artifact from the PR #84 dependency merge, not a recurring issue.
