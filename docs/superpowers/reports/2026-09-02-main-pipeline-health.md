# `main` Branch CI/CD Pipeline Health Analysis

- Repo: byx-darwin/go-tools
- Trigger context: `feat/76-jwt-secret-strength` merged locally into `main` at `56383ff` (`Merge branch 'feat/76-jwt-secret-strength'`); **push to remote is still pending** at analysis time.
- Scope: this report analyzes the pipeline health of the **currently pushed** `main` (remote HEAD `fdb7c04`, PR #86 merge) — the 30-day run history GitHub Actions has recorded so far. No CI run yet exists for the pending `56383ff` merge; it will run on push.
- Analysis date: 2026-09-02
- Tooling: `gf pipeline report/status/jobs/logs` (read-only)

## 1. Success rate

| Window | Runs | Success rate |
|---|---|---|
| 7 days | 35 | 74.3% |
| 14 days | 40 | 72.5% |
| 30 days | 43 | 69.8% |

**Trend: improving but still below a healthy bar.** The 7-day rate (74.3%) is higher than the 30-day rate (69.8%), meaning the failure-heavy period is concentrated further back in the window and recent days trended cleaner — but even the best recent window sits well under the conventional 90%+ "healthy" threshold. 🟡 **Alert: sustained success rate < 80%.**

Of the last 30 `main` runs (2026-08-28 → 2026-09-02), 6 concluded `failure`:

| Run ID | Time (UTC) | Failing jobs |
|---|---|---|
| 33293729371 | 08-30 05:00 | `govulncheck (go-framework)`, `govulncheck (go-middleware)` |
| 33372548489 | 08-31 08:21 | `govulncheck (go-framework)`, `govulncheck (go-middleware)` |
| 33490082361 | 09-01 09:01 | `go-middleware`, `go-framework` |
| 33496010087 | 09-01 10:10 | `go-middleware`, `go-framework` |
| 33496823343 | 09-01 10:19 | `go-middleware`, `go-framework` |
| 33576217060 | 09-02 00:39 | `govulncheck (go-framework)`, `govulncheck (go-middleware)` |

## 2. Failure patterns

Two distinct, recurring root causes account for **100%** of the 6 failed runs — no unrelated/one-off failures observed.

### 2.1 `govulncheck` dependency-lag failures (3 occurrences: 08-30, 08-31, 09-02)

Both `govulncheck (go-framework)` and `govulncheck (go-middleware)` fail together each time, with the same two CVEs every time:

- **GO-2026-6061** — `google.golang.org/grpc`: found `v1.81.1`, fixed in `v1.82.1` (affects `go-framework`)
- **GO-2026-5970** — `golang.org/x/text`: found `v0.38.0`, fixed in `v0.39.0` (affects `go-middleware`, reached via `tls/producer.go` init)

Pattern: a scan finds the vulnerable version, then the **very next paired run (minutes later)** is green once the dependency bump commit lands. This happened on 08-30, again on 08-31, and again on 09-02 (00:39 fail → 00:42/00:48 pass) — i.e. it is a **recurring propagation-lag flake**, not a one-off. Each time, the fix (bumping the dependency) was applied within ~10 minutes, but the pattern itself repeats every 1-2 days, suggesting `go.work.sum`/`go.mod` bump commits and the scan trigger are racing rather than being sequenced.

**Assessment: recurring, self-resolving, but not yet addressed at the process level** — this is the single largest contributor to the 30-day failure count (3 of 6 failures).

### 2.2 `go mod tidy -diff` cross-module version drift (3 occurrences, all within 90 minutes: 09-01 09:01–10:19)

`go-middleware` and `go-framework` CI jobs failed together three times in a row. Root cause (confirmed from job logs, run 33490082361):

```
go: finding module for package github.com/byx-darwin/go-tools/go-auth/revocation
go: downloading github.com/byx-darwin/go-tools v0.2.2
go: github.com/byx-darwin/go-tools/go-middleware/auth imports
    github.com/byx-darwin/go-tools/go-auth/revocation: module github.com/byx-darwin/go-tools/go-auth@latest found (v0.1.0), but does not contain package github.com/byx-darwin/go-tools/go-auth/revocation
##[error]Process completed with exit code 1.
```

`go-middleware/auth` imports `go-auth/revocation`, a package that exists in the workspace checkout (via `go.work`) but had **not yet been tagged/published** as part of a `go-auth` release. The `go mod tidy -diff` CI step resolves against the published module registry (not `go.work`), so it correctly failed until a `go-auth` tag containing the `revocation` package was cut. All three failures cluster in a single 78-minute window before self-resolving — consistent with an in-progress release sequencing issue rather than a code defect.

**Assessment: process gap, not flaky** — `go mod tidy -diff` will always fail this way whenever a downstream module (`go-middleware`/`go-framework`) is updated to depend on an unreleased `go-auth` package before that package is tagged. This is a repeatable failure mode under the current release ordering, worth a workflow fix (see §5).

## 3. Duration distribution / bottlenecks

30-day average run duration: **~160s** (2m40s) for the fast/inner jobs; the CI workflow's total wall time is dominated by one job.

Latest green run (33580255767, 2026-09-02T01:39):

| Job | Duration | Notes |
|---|---|---|
| `go.mod hygiene` | 5s | fastest |
| `go-auth` | 48s | |
| `go-common` | 1m38s | |
| `go-framework` | 2m40s | |
| **`go-middleware`** | **4m36s** | longest job, drives total wall time |

`go-middleware` is consistently the slowest job across recent runs (4m36s–4m53s), matching its historical baseline — it has the most external client integrations (redis/kafka/db/es/clickhouse/tls). This is a stable characteristic, not a new regression, but it remains the top duration-optimization target if pipeline wall-clock time becomes a priority (e.g. parallelizing its sub-packages' tests, or splitting the `-race` test run from lint/build).

## 4. Flaky tests

No flaky **test** behavior detected (no intermittent test pass/fail on identical code across retries). However, two **flaky pipeline steps** were identified at the CI/infra level:

- `govulncheck (go-framework)` / `govulncheck (go-middleware)` — intermittently red/green depending on whether a dependency-bump commit has landed yet relative to the scan (§2.1). Recurs every 1-2 days.
- `go mod tidy -diff` (in `go-middleware`/`go-framework` jobs) — fails whenever cross-module dependency publication lags code that already references the new package (§2.2).

Neither is a genuine Go test flake (`*_test.go` logic); both are **release/dependency-sequencing races** surfaced by CI gates.

## 5. Suggestions (priority order)

1. **[High] Sequence `go-auth` tag cuts before dependent modules merge new imports.** The 09-01 incident (§2.2) is fully preventable: land/tag `go-auth` package additions (e.g. `revocation`) *before* merging the `go-middleware`/`go-framework` PR that imports them, or gate the dependent PR's merge on the tag existing. This removes a 100%-reproducible 3-failure cluster.
2. **[Medium] Decouple dependency-bump commits from the `govulncheck` trigger window** (§2.1). Options: (a) run `govulncheck` as a required-status check on the *same* commit that bumps the vulnerable dependency (already true for `push`-to-`main`, but the bump and the scan appear to land as separate back-to-back commits) — consider bundling the version bump into the same commit/PR that introduced the now-flagged dependency; (b) treat a `govulncheck` failure on `main` as a P1 auto-notify (e.g. Slack/Issue) rather than relying on the next manual push to self-heal, since GO-2026-6061/5970 recurred across 3 separate days.
3. **[Low] Consider trimming `go-middleware`'s CI wall time** (§3) — it is consistently 1.5-2x the next-slowest job. Not urgent (no regression), but worth revisiting if overall pipeline latency becomes a developer-experience concern (e.g. run `-race` only on changed packages, or split redis/kafka/db/es/clickhouse/tls into parallel matrix jobs).
4. **[Info, no action] Confirm the pending `feat/76-jwt-secret-strength` push triggers a clean run.** The local merge commit `56383ff` has not yet been pushed; once pushed, verify the resulting `main` CI run is green and that it doesn't hit either failure pattern above (it does not touch `go.mod`/`go.sum` per the branch's commit list, so `security.yml`'s path-filtered govulncheck should not even fire — see the PR #86 report §4 for the same path-filter behavior).

## Summary

| Dimension | Finding |
|---|---|
| Success rate | 69.8% (30d) / 74.3% (7d) — 🟡 below the 80% health threshold, improving trend |
| Failure patterns | 100% of 6 failures explained by 2 recurring causes: govulncheck dependency-lag (3×) and go-auth/go-middleware version-drift in `go mod tidy -diff` (3×, single incident window) |
| Duration | ~160s avg; `go-middleware` job (4m36s–4m53s) is the stable bottleneck, not a new regression |
| Flaky tests | No genuine test flakiness; 2 flaky **CI gate** patterns tied to release/dependency sequencing (see above) |
| Pending action | `feat/76-jwt-secret-strength` merge (`56383ff`) not yet pushed — no CI data for it yet; expected to run clean per commit content (no go.mod/go.sum changes) |
