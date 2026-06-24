# Agent Engineering Rules

This file defines how an AI coding agent should work in this repository.
It focuses on execution quality, validation discipline, and risk control.

## 1. Goals

- Make small, correct, explainable changes.
- Prefer fast, reliable validation over guesswork.
- Keep repository behavior and public API contracts stable unless the task explicitly changes them.
- Avoid risky actions unless the user explicitly asked for them.

## 2. Core Rules

- MUST read relevant implementation and nearby tests before editing.
- MUST keep diffs minimal and avoid unrelated cleanup.
- MUST validate changes with the smallest useful checks first.
- MUST update tests when behavior changes.
- MUST explain what changed, what was run, and any remaining risk.
- MUST NOT install dependencies, deploy, change external state, or run destructive operations without explicit permission.

## 3. Information Gathering

- Confirm the target symbol, file, or contract exists before editing.
- Read the narrowest set of files that can justify the change safely.
- Prefer existing helpers, patterns, and tests over inventing new structures.
- Stop exploring when additional reading no longer changes the implementation plan.
- If behavior is ambiguous after reasonable inspection, ask instead of guessing.

## 4. Change Strategy

- Prefer the smallest possible edit that fully solves the requested task.
- Preserve existing public behavior unless the task explicitly changes it.
- Do not mix refactors with behavior changes unless necessary.
- Keep exported APIs, module interfaces, and cross-module contracts especially stable.
- When editing shared helpers or utilities used by multiple modules, assume broader regression risk and validate accordingly.

## 5. Testing Strategy

### 5.1 Unit Tests

Use unit tests for helpers, pure logic, formatting, parsing, output building, and other isolated behavior.

- SHOULD add or update unit tests whenever a logic branch changes.
- SHOULD prefer helper-level coverage instead of relying only on higher-level integration tests.

### 5.2 Integration Tests

Use integration tests for cross-module wiring, configuration loading, or flows that wire multiple components together.

- MUST run relevant integration tests when public contracts, structured outputs, or module interfaces may be affected.

### 5.3 Build Verification

Always verify that the workspace builds after changes:

- `go build ./...` to verify all modules compile
- `go vet ./...` to check for common issues
- `golangci-lint run` per module to check static analysis (see `.golangci.yml`)

## 6. Validation Order

Run validation from smallest and fastest to broadest and slowest:

1. single test function or focused unit tests
2. relevant test file
3. relevant package or component tests
4. related integration tests
5. broader repository checks only when needed

For final PR-quality validation, use the workspace-wide checks:

- `go build ./...`
- `go vet ./...`
- `golangci-lint run` (per module, see `.claude/rules/go.md` § 8.6)
- `go test ./... -count=1`

## 7. Failure Handling

- If validation fails, make the smallest plausible fix and rerun the most relevant check.
- Do not stack multiple speculative fixes before rerunning.
- If repeated attempts do not improve the situation, stop and ask for clarification.
- MUST NOT bypass failing validation by silently skipping tests or reducing coverage without explanation.

## 8. Documentation Rules

- When public APIs, configuration options, or module behavior change, update docs and godoc comments.
- Prefer one clear source of truth over repeating the same contract in multiple places.

## 9. Repository-Specific Guidance

- Module-specific changes SHOULD be validated with targeted tests for that module.
- Cross-module changes MUST verify the full workspace builds and tests pass.
- Shared utility refactors in `tools/` SHOULD add package-level unit tests.
- Config changes SHOULD be validated conservatively because they affect downstream services.

## 10. Communication

- State the plan before major edits or validation runs.
- Summarize progress as the task advances.
- On completion, report:
  - what changed
  - what tests or checks were run
  - any follow-up risk or optional next step

## 11. Stop Conditions

- Stop when the requested task is complete and relevant validation has passed.
- Do not continue into adjacent improvements without asking.
- If a next step is helpful but outside scope, suggest it rather than doing it automatically.
