# gf-review: PR #102 — feat(go-framework): TLS-by-default + Functional Options to observability providers

- Repo: byx-darwin/go-tools
- PR: #102, branch `feat/96-observability-otel-tls-options` → `main`
- Closes: #96
- Review date: 2026-09-03
- Reviewer: independent delivery-review gate (post subagent-driven-development + prior opus-tier whole-branch review)

## Method

This is an independent gate on top of the prior internal review chain (4 task-level
reviews + one opus-tier whole-branch review that found and fixed 3 Important
findings). Rather than defer to that prior conclusion, I:

1. Read the full PR diff (`gh pr diff 102`, 2130 lines) directly.
2. Fetched the PR head into an isolated git worktree (`/tmp/pr102-wt`) and
   independently ran:
   - `go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...` — clean
   - `go vet` (same scope) — clean
   - `gofmt -l` over all tracked `.go` files — clean
   - `golangci-lint run --timeout=5m ./<module>/...` for all 4 modules — 0 issues each
   - `go test ./go-framework/... -count=1` — all packages pass
3. Grepped for other `NewProvider(` call sites and remaining hardcoded
   `otlp*grpc.WithInsecure()` usages to check the change was applied
   consistently everywhere it needed to be.

## Findings

None. No Important or Minor findings from this independent pass.

Specific things checked and found correct:

- **TLS default behavior**: `tlsDialCredentials` priority is
  `WithTLSConfig` > `cfg.Insecure` > default TLS
  (`tls.Config{MinVersion: tls.VersionTLS12}`, system root cert pool via nil
  `RootCAs`), implemented identically and correctly in both
  `go-framework/hertz/observability/provider.go` and
  `go-framework/kitex/observability/provider.go`. `MinVersion: TLS12` is a
  reasonable hardening default (better than the plan doc's original bare
  `tls.Config{}` sketch — this was tightened in the actual implementation).
- **Backward compatibility**: `NewProvider(ctx, cfg, opts ...Option)` keeps
  existing 2-arg call sites (`example/main.go`, `go-framework/kitex/observability/doc.go`
  godoc example) compiling unchanged — verified via successful build, not just
  by reading the diff.
- **Call-site wiring completeness**: grepped all `NewProvider(` call sites —
  `go-framework/hertz/server.go` (legacy Jaeger path) correctly plumbs the new
  `JaegerOption.Insecure` field into `config.ObservabilityConfig.Insecure`;
  `example/main.go`'s two call sites use `cfg.Observability` directly, which
  already carries the new field. No remaining hardcoded
  `otlptracegrpc.WithInsecure()` / `otlpmetricgrpc.WithInsecure()` call sites
  outside the new priority-selection helpers.
- **Test coverage**: table-driven `TestTLSDialCredentials_Priority` (both
  packages) tests the priority logic directly rather than only through
  `NewProvider`'s error return (correctly reasoned in test comments — OTLP
  gRPC's lazy-connect means construction-time errors wouldn't catch a wrong
  credential choice). `WithMetricExporter` now has real injection-based
  coverage via a hand-rolled `fakeMetricExporter` test double, addressing what
  the PR description calls out as previously-untested.
- **Options pattern compliance**: `Option`/`providerOptions`/`WithXxx`
  functions match `.claude/rules/options-pattern.md` (nil-guarded setters,
  each function sets one field, godoc present).
- **Lint/format compliance**: godoc comments present on all new exported
  symbols (`Option`, `WithTraceExporter`, `WithMetricExporter`, `WithSampler`,
  `WithPropagator`, `WithTLSConfig`, `ObservabilityConfig.Insecure`,
  `JaegerOption.Insecure`); no errcheck violations; gofmt-clean.
- **Scope discipline**: the design doc explicitly and correctly scopes out
  refactoring the `otel.GetTracerProvider()`/`GetMeterProvider()`/
  `GetTextMapPropagator()` global-singleton read sites (`tracer.go`,
  `client.go`, `suite.go`, `grpc_metadata.go`) as a separate, larger concern —
  consistent with what the diff actually touches (no unrelated changes crept
  in).
- **Documented breaking change**: the OTLP TLS-by-default flip is called out
  in the PR body, `CLAUDE.md` D7, `example/config.yaml` inline comment, and
  package-level godoc — appropriately visible for a behavior change that can
  break deployments pointed at plaintext collectors.

## Verdict

**Approve (submitted as `comment` verdict).** No findings from this
independent pass; build/vet/lint/test all verified green directly against the
PR head (not just taken on the author's word), and the TLS priority logic,
backward compatibility, and call-site wiring were independently traced
through the code rather than assumed from the prior review's summary.

Note: `gf review approve 102` failed — GitHub rejects approve/request-changes
on one's own PR (author `byx-darwin` matches the authenticated `gf`/git
user). Submitted via `gf review comment 102` instead, recording the same
"no findings, ready to merge" verdict as a comment. Review posted:
https://github.com/byx-darwin/go-tools/pull/102 (comment id 5099922882,
2026-09-03T09:01:58Z).
