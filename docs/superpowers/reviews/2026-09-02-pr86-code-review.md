# Code Review Report — PR #86 (Issue #75)

**Title:** fix(auth): WithIssuer 在 Verify 路径未生效，存在越权风险
**Branch:** `feat/75-jwt-verify-issuer` → `main`
**Merge commit:** `fdb7c04` (already merged at review time)
**Reviewer:** Claude (independent verification pass, Phase 4 gf-workflow record)
**Review type:** Written report only — **not submitted via `gf review`**

## Self-review disclosure

The authenticated `gf` account (`byx-darwin`) is the same as the PR author
(`byx-darwin`). Per `gf-review`'s self-review prohibition, no formal verdict
was (or could be) submitted to the PR. This document is an independent
written analysis for the Issue #75 / PR #86 record, produced per the
Phase 4 gf-workflow instruction to run the review process anyway even
though prior internal review already occurred.

## Scope reviewed

Diff of PR #86 (`git diff 6b8e21c..fdb7c04`, 6 files changed):

- `go-auth/jwt/options.go` — new `WithExpectedIssuer` option, `config.expectedIssuer` field, godoc updates on `WithIssuer`
- `go-auth/jwt/token.go` — `Verify` now passes `gojwt.WithIssuer(cfg.expectedIssuer)` as a `ParserOption` when set
- `go-auth/jwt/token_test.go` — 6 new tests (4 direct Verify cases + 2 Refresh-coexistence cases)
- `example/handler/auth_jwt.go` — verify handler now passes `WithExpectedIssuer(jwtIssuer)`
- `docs/superpowers/plans/...` and `docs/superpowers/specs/...` — planning artifacts (docs-only, no review concerns)

## Verification performed (independent, not just re-reading the PR description)

1. `git fetch`/`git pull --ff-only` to bring local `main` to the actual merge commit (local tree was 4 commits behind) — confirmed the diff reviewed matches what's live on `main`.
2. `go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... ./example/...` — clean.
3. `go vet ./go-auth/...` — clean.
4. `gofmt -l` across the whole repo — clean.
5. `golangci-lint run ./go-auth/...` — 0 issues.
6. `go test ./go-auth/jwt/... -run Issuer -v` — all 9 issuer-related tests pass, including the two Refresh-coexistence tests (`TestRefreshWithIssuerAndExpectedIssuerCoexist`, `TestRefreshWithExpectedIssuerMismatchFails`) added during the internal fix round.
7. `go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1` — full workspace suite, all packages pass, no regressions.
8. Read `golang-jwt/jwt/v5@v5.3.1` source directly (`parser_option.go`, `validator.go`) rather than trusting the PR's claim: confirmed `WithIssuer(iss string) ParserOption` sets `validator.expectedIss`, and `verifyIssuer(claims, expectedIss, required=true)` is called with `required=true`, which errors on a **missing** `iss` claim, not just a mismatched one. This independently confirms the "missing claim → error" behavior the PR and its tests assert.
9. Confirmed `mapJWTError` in `token.go` has a catch-all default branch (`errors.Is(err, gojwt.ErrTokenExpired)` → `ErrTokenExpired`, else → `ErrTokenInvalid`), so the new issuer-validation error path is correctly covered without needing a new error code — matches what the tests assert (`autherror.CodeTokenInvalid`).
10. Confirmed `WithIssuer` is functionally unchanged (only its godoc gained a clarifying note) by diffing the function body — still a plain `if issuer != "" { c.issuer = issuer }`, only read by `setClaimsDefaults` which is Sign-only.
11. Checked `go-framework/hertz/middleware/jwt_auth.go:62` — confirmed it still calls `gojwt.Verify[T](token, secret)` with no options passthrough, i.e. the Hertz `JWTAuth` middleware genuinely cannot adopt `WithExpectedIssuer` yet. This validates the PR's claim that spinning this out to Issue #85 (rather than fixing it here) is accurate scoping, not scope-avoidance — the fix is a separate, larger option-passthrough API design.
12. Confirmed the 3 previously-documented internal-review fixes are actually present in the merged code:
    - Refresh-coexistence test: present (`TestRefreshWithIssuerAndExpectedIssuerCoexist`, `TestRefreshWithExpectedIssuerMismatchFails`), passing.
    - Empty-string fail-open now documented: `WithExpectedIssuer`'s godoc explicitly says "空字符串忽略（即不做任何 issuer 校验，请确保配置项非空，否则本选项等同未启用）".
    - Example app now demonstrates the option: `example/handler/auth_jwt.go:118` passes `jwt.WithExpectedIssuer(jwtIssuer)`.

## Findings

No Critical or Important findings beyond what the PR's own internal review process already identified and fixed.

**One new Minor/nit finding**, not previously documented:

- **`go-auth/jwt/token.go:55-59` (Minor, discoverability nit):** `Verify`'s own godoc comment was not updated to mention `WithExpectedIssuer`. It documents `WithSigningMethod` but the new issuer-validation option is only documented on the option's own godoc (`WithExpectedIssuer` in `options.go`), not cross-referenced from `Verify` itself. A caller reading only `Verify`'s doc (e.g. via `go doc jwt.Verify`) wouldn't discover the option exists. Same severity class as the "example not demonstrating the new option" issue already fixed in this PR — suggests one more doc pass would be warranted, but does not affect correctness, security, or test coverage.

No other issues found across correctness, security, API design, backward compatibility, or test coverage dimensions.

## Assessment by dimension

| Dimension | Verdict |
|---|---|
| Correctness | ✅ Verified independently against golang-jwt v5.3.1 source; issuer check runs after signature verification (inside `ParseWithClaims`'s post-parse validator step), no bypass path |
| Security (the actual point of the PR) | ✅ Closes the privilege-escalation gap described in #75 — `Verify` now actually enforces issuer when asked, and fails closed on a missing `iss` claim (not silently accepted) |
| Backward compatibility | ✅ `WithIssuer` unchanged; `Verify` without `WithExpectedIssuer` still ignores issuer entirely (explicit regression test) |
| `Refresh` interaction | ✅ Independently re-run; both coexistence and mismatch-failure paths pass |
| API design (Options pattern) | ✅ Follows `.claude/rules/options-pattern.md` — single-field `WithXxx`, empty-string-ignored guard |
| Test coverage | ✅ Match / mismatch / missing-claim / backward-compat / Refresh-coexist / Refresh-mismatch — 6 new tests, all passing |
| Lint / build / vet | ✅ Clean across build, vet, gofmt, golangci-lint |
| Scope discipline | ✅ Issue #85 (Hertz middleware passthrough) correctly deferred — verified the middleware genuinely has no options passthrough today, so this wasn't scope-avoidance |
| Docs | ⚠️ One minor gap: `Verify`'s own godoc doesn't cross-reference `WithExpectedIssuer` (see Findings) |

## Verdict

**Would approve.** The fix is correct, the security claim is verified against actual library source (not just asserted), backward compatibility is preserved and tested, and the previously-identified internal review findings are confirmed fixed in the merged code. One minor documentation nit was found beyond what was already documented — worth a follow-up one-line godoc addition on `Verify`, but not blocking and not warranting a request-changes verdict.

**Verdict not submitted to GitHub** — self-review (PR author == authenticated `gf` account `byx-darwin`) per `gf-review` skill's non-negotiable self-review prohibition. This report stands as the independent record for the PR/Issue.
