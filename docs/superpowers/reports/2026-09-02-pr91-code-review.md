# Code Review Report — PR #91

**Title:** feat(hertz): JWTAuth 中间件支持透传 Verify 选项
**Repo:** byx-darwin/go-tools
**Branch:** `feat/85-jwt-verify-issuer` → `main`
**Closes:** #85
**Reviewer:** Formal Phase 4 delivery-gate review (gf-review), independent pass
**Date:** 2026-09-02

## Scope

Independent review of the current PR diff (4 commits: docs/spec+plan, feature
implementation, docs follow-up on HMAC-only limitation, and an unrelated
`chore: fix pre-commit formatting issues` commit). This review does not rely
on the earlier informal `superpowers:requesting-code-review` pass result and
re-derives its own verdict from the diff and local verification.

## What changed

- `go-framework/hertz/middleware/jwt_auth.go`:
  - `jwtAuthConfig` gains `verifyOptions []gojwt.Option`.
  - New `WithVerifyOptions(opts ...gojwt.Option) JWTAuthOption`, generic
    passthrough (not a single-purpose `WithExpectedIssuer` shortcut), styled
    consistently with the existing `WithRevocationChecker`.
  - `JWTAuth[T]` internal call changed from `gojwt.Verify[T](token, secret)` to
    `gojwt.Verify[T](token, secret, cfg.verifyOptions...)`.
  - Godoc updated on both the new option and `JWTAuth`, including an explicit
    warning that passing `gojwt.WithSigningMethod` through this option will
    fail closed with `ErrJWTKeyTypeMismatch` because the middleware's `secret`
    parameter is fixed to `[]byte` (HMAC only) — this note was added in a
    follow-up commit (`e10f8e6`) addressing the one Minor finding from the
    earlier informal review pass.
- `go-framework/hertz/middleware/jwt_auth_test.go`: three new tests —
  issuer-mismatch rejection (401), issuer-match success, and a
  no-`WithVerifyOptions` regression test proving unchanged default behavior.
- `docs/superpowers/specs/...` and `docs/superpowers/plans/...`: new design
  and implementation-plan docs for this change (process artifacts).
- Unrelated formatting fixes (separate `chore` commit): `goimports` grouping
  fixes on generated Kitex client files and `hertz_response.pb.go`, a trailing
  newline fix in `example/test/report.md`, and adding a trailing newline to
  `.claude/settings.json`. Called out as pre-existing repo-wide pre-commit
  issues, unrelated to Issue #85, isolated into its own commit.

## Independent verification performed

Checked out the PR branch (existing worktree at
`.worktree/feat/85-jwt-verify-issuer`, HEAD `e10f8e6`) and ran, independent of
the PR description's claims:

```
go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...   → clean
go vet   ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...   → clean
gofmt -l go-framework/hertz/middleware/jwt_auth.go go-framework/hertz/middleware/jwt_auth_test.go → no output
golangci-lint run --timeout=5m ./go-framework/...                               → 0 issues
go test ./go-framework/hertz/middleware/... -run TestJWTAuth -v                 → 13/13 PASS
```

All claims in the PR's "Test plan" checklist were reproduced locally and
confirmed accurate.

## Analysis

### Correctness
- `cfg.verifyOptions` defaults to a nil slice; `gojwt.Verify[T](token, secret,
  cfg.verifyOptions...)` with a nil/empty slice is behaviorally identical to
  the prior no-variadic-args call — confirmed by the new
  `TestJWTAuth_NoVerifyOptions_BehaviorUnchanged` regression test and by
  reading `go-auth/jwt.applyOptions`, which only mutates state when options
  are present.
- The generic-passthrough design (vs. a dedicated `WithExpectedIssuer`
  wrapper) is the right call: it transparently forwards any current or future
  `go-auth/jwt.Option` (e.g. `WithExpectedIssuer`, `WithSigningMethod`)
  without the middleware needing to keep re-wrapping each new upstream option.
- Multiple `WithVerifyOptions` calls append in order, matching documented
  semantics and being equivalent to calling `gojwt.Verify` directly with the
  same option list.
- The godoc addition about `WithSigningMethod` failing closed with
  `ErrJWTKeyTypeMismatch` is accurate: `go-auth/jwt.validateKeyType` requires
  `[]byte` for the HMAC family and returns that exact error for any other key
  type when a non-HMAC `SigningMethod` is selected while `JWTAuth[T]`'s
  `secret` stays `[]byte`. This is a correct and honest characterization
  (fail-closed, not a silent bypass).

### API / consistency
- New option follows the existing `Option` pattern in this file
  (`WithXxx(...) JWTAuthOption`, single responsibility, godoc present) and the
  repo's documented Functional Options rules.
- No breaking change to `JWTAuth[T](secret []byte, opts ...JWTAuthOption)`
  signature; purely additive.

### Tests
- Coverage is appropriately narrow and targeted: mismatch (401), match (200 +
  claims propagate through `GetClaims`), and a behavior-unchanged regression
  when the option is not used — mirrors the precedent set by
  `TestJWTAuth_NoRevocationChecker_BehaviorUnchanged`.
- Tests exercise real `Sign`/`Verify` end-to-end per repo test convention (no
  mocking of JWT internals).
- No new test for calling `WithVerifyOptions()` with zero arguments, but this
  is a trivial no-op path (`append(nil)` is a no-op) and not worth a dedicated
  test.

### Documentation
- `JWTAuth` and `WithVerifyOptions` godoc both updated per the exported-symbol
  comment requirement; usage examples included.
- The HMAC-only caveat is now documented directly at the point of use
  (`WithVerifyOptions` godoc), which is the right place — not buried in a
  separate doc.

### Process / diff hygiene
- The unrelated formatting commit mixes non-behavioral `goimports` cleanup
  into this PR. It's isolated to its own commit and explicitly called out in
  the PR body as pre-existing/unrelated, which is reasonable mitigation, but
  strictly it's still scope creep against `.claude/rules/go.md` §9 ("Avoid
  mixing docs-only, refactor-only, and behavior changes without need") and
  `.claude/rules/agent-engineering.md` §4. Not a blocker — verified those
  files are formatting-only (import grouping / trailing newline), not
  behavioral, git-diff confirmed no logic changes in
  `example/kitex_generated/...`, `go-framework/hertz/hertz_response.pb.go`,
  `example/test/report.md`, `.claude/settings.json`.

## Findings

No new Critical or Important issues found beyond what the earlier informal
review pass identified (that pass's 1 Minor doc-clarity finding is confirmed
fixed in commit `e10f8e6`).

1 additional Minor/nit observation (non-blocking):

- **[Minor / nit] Unrelated formatting commit riding along in this PR.** The
  `chore: fix pre-commit formatting issues` commit touches files unrelated to
  Issue #85 (`example/kitex_generated/*`, `go-framework/hertz/hertz_response.pb.go`,
  `example/test/report.md`, `.claude/settings.json`). Content is confirmed
  non-behavioral (import grouping / trailing newline only). Recommendation
  for future PRs: land such repo-wide pre-commit hygiene fixes as their own
  standalone PR rather than bundled with a feature PR, per repo diff-hygiene
  rules. Not a reason to block this PR — already isolated to a separate
  commit and disclosed in the PR description.

## Verdict

**Approve.** Implementation is correct, backward-compatible (verified by
regression test and by reading upstream option-application code), well
tested, and well documented, including proactively documenting a real
non-obvious limitation (HMAC-only via this passthrough) that the informal
pre-merge pass flagged and that has since been fixed. Full validation suite
(build/vet/gofmt/lint/test) reproduced independently and green. The one
observation above (unrelated formatting commit) is a minor process note, not
a code defect, and does not block merge.
