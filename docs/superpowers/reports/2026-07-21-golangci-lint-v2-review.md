# Review: chore(ci): align golangci-lint to v2 (#32 / PR #35)

- **Date**: 2026-07-21
- **Mode**: fast
- **Reviewer**: orchestrator (gitflow-workflow)

## Diff Summary

4 files changed, 18 insertions(+), 9 deletions(-)

| File | Change |
|------|--------|
| `.github/workflows/ci.yml` | Re-enable golangci-lint step: action@v9, v2.12.2, repo-root execution, removed outdated comments |
| `go-auth/jwt/token.go:128` | `reflect.Ptr` → `reflect.Pointer` (deprecated alias, value=16 unchanged) |
| `README.md` | Added golangci-lint v2 (>= v2.12.2) to Requirements with install command |
| `.claude/rules/go.md` §8.6 | Added required version note with upgrade command |

## Findings

**None.** All changes are minimal, focused, and match the design doc and plan.

## Verification Results

| Check | go-common | go-auth | go-middleware | go-framework |
|-------|-----------|---------|---------------|--------------|
| golangci-lint v2.12.2 | 0 issues | 0 issues | 0 issues | 0 issues |
| go build | pass | pass | pass | pass |
| go test -count=1 | pass | pass | pass | pass |
| CI (GitHub Actions) | SUCCESS | SUCCESS | SUCCESS | SUCCESS |

## Dogfooding Checklist

- [x] Documented upgrade command works: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`
- [x] Documented lint loop works: `for m in go-common go-auth go-middleware go-framework; do golangci-lint run --timeout=5m ./$m/...; done` → all 0 issues
- [x] CI lint step executes and passes on all 4 matrix modules
- [x] `reflect.Ptr` → `reflect.Pointer` is value-identical (const 16), no behavior change confirmed by tests

## Acceptance Criteria (#32)

- [x] Chose "upgrade to v2" (option 1)
- [x] CI uses correct version (v2.12.2 via action@v9)
- [x] Lint loop passes on all 4 modules including go-auth
- [x] README and rules document required version

## Verdict

**APPROVE** — low-risk, well-verified change. Ready to merge.
