# Release Versioning Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove `replace` directives and `v0.0.0` pins from published module go.mod files so external consumers can resolve them, and establish release tooling/documentation.

**Architecture:** Clean go.mod approach — published modules (go-common, go-auth, go-middleware, go-framework) contain no `replace` directives and reference real published versions. Local development relies on `go.work` workspace resolution. A release script automates tagging, CI prevents regression, and RELEASE.md documents the process.

**Tech Stack:** Go 1.26 modules, go.work workspace, GitHub Actions CI, bash scripting

## Global Constraints

- Go 1.26.5 (workspace mode via `go.work`)
- Tag convention: `<module>/v<major>.<minor>.<patch>` (e.g., `go-common/v0.1.0`)
- Published modules: go-common, go-auth, go-middleware, go-framework
- `example/` module is internal-only — keeps `replace` directives
- First version: v0.1.0 for all four modules
- golangci-lint v2 must pass per module
- All exported symbols need godoc comments

---

### Task 1: Clean go.mod Files

**Files:**
- Modify: `go-auth/go.mod`
- Modify: `go-middleware/go.mod`
- Modify: `go-framework/go.mod`
- Modify: `go-auth/go.sum` (regenerated)
- Modify: `go-middleware/go.sum` (regenerated)
- Modify: `go-framework/go.sum` (regenerated)

**Interfaces:**
- Consumes: Nothing (first task)
- Produces: Clean go.mod files with `v0.1.0` sibling requires, no `replace` directives

- [ ] **Step 1: Edit go-auth/go.mod — remove replace, update require**

In `go-auth/go.mod`, change:
```
require (
	github.com/byx-darwin/go-tools/go-common v0.0.0
```
to:
```
require (
	github.com/byx-darwin/go-tools/go-common v0.1.0
```

And delete the line:
```
replace github.com/byx-darwin/go-tools/go-common => ../go-common
```

- [ ] **Step 2: Edit go-middleware/go.mod — remove replace block, update requires**

In `go-middleware/go.mod`, change:
```
	github.com/byx-darwin/go-tools/go-auth v0.0.0
	github.com/byx-darwin/go-tools/go-common v0.0.0
```
to:
```
	github.com/byx-darwin/go-tools/go-auth v0.1.0
	github.com/byx-darwin/go-tools/go-common v0.1.0
```

And delete the replace block:
```
replace (
	github.com/byx-darwin/go-tools/go-auth => ../go-auth
	github.com/byx-darwin/go-tools/go-common => ../go-common
)
```

- [ ] **Step 3: Edit go-framework/go.mod — remove replace block, update requires**

In `go-framework/go.mod`, change:
```
	github.com/byx-darwin/go-tools/go-auth v0.0.0-00010101000000-000000000000
	github.com/byx-darwin/go-tools/go-common v0.0.0
```
to:
```
	github.com/byx-darwin/go-tools/go-auth v0.1.0
	github.com/byx-darwin/go-tools/go-common v0.1.0
```

And delete the replace block:
```
replace (
	github.com/byx-darwin/go-tools/go-auth => ../go-auth
	github.com/byx-darwin/go-tools/go-common => ../go-common
)
```

- [ ] **Step 4: Run go mod tidy in each affected module**

Run:
```bash
cd go-auth && go mod tidy && cd ..
cd go-middleware && go mod tidy && cd ..
cd go-framework && go mod tidy && cd ..
```
Expected: go.sum files updated, no errors (workspace resolves siblings locally via go.work `use`)

- [ ] **Step 5: Verify workspace builds**

Run:
```bash
go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...
```
Expected: BUILD SUCCESS (no output)

- [ ] **Step 6: Verify go vet passes**

Run:
```bash
go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...
```
Expected: No output (success)

- [ ] **Step 7: Verify tests pass**

Run:
```bash
go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1
```
Expected: All tests pass

- [ ] **Step 8: Verify no replace remains in published modules**

Run:
```bash
grep -l '^replace' go-common/go.mod go-auth/go.mod go-middleware/go.mod go-framework/go.mod 2>/dev/null || echo "CLEAN: no replace directives"
```
Expected: `CLEAN: no replace directives`

- [ ] **Step 9: Commit**

```bash
git add go-auth/go.mod go-auth/go.sum go-middleware/go.mod go-middleware/go.sum go-framework/go.mod go-framework/go.sum
git commit -m "chore(release): remove replace directives, pin siblings to v0.1.0 (#29)

Remove replace directives from go-auth, go-middleware, go-framework.
Set sibling module requires to v0.1.0 for independent publishability.
Local dev continues via go.work workspace resolution.

Closes #29"
```

---

### Task 2: Create Release Script

**Files:**
- Create: `scripts/release.sh`

**Interfaces:**
- Consumes: Nothing (standalone utility)
- Produces: `scripts/release.sh` — automates tag creation for future releases

- [ ] **Step 1: Create scripts/release.sh**

```bash
#!/usr/bin/env bash
# release.sh — Create and push a module release tag.
#
# Usage:
#   ./scripts/release.sh <module> <version>
#
# Examples:
#   ./scripts/release.sh go-common v0.2.0
#   ./scripts/release.sh go-framework v1.0.0
#
# Pre-conditions:
#   - Module directory exists
#   - Working tree is clean
#   - Tag does not already exist
#   - Module builds and tests pass
set -euo pipefail

MODULE="${1:-}"
VERSION="${2:-}"

if [[ -z "$MODULE" || -z "$VERSION" ]]; then
    echo "usage: $0 <module> <version>"
    echo "example: $0 go-common v0.2.0"
    exit 1
fi

# Validate version format
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]]; then
    echo "error: version must match v<major>.<minor>.<patch>[-prerelease]"
    echo "  got: $VERSION"
    exit 1
fi

# Validate module directory
if [[ ! -d "$MODULE" ]]; then
    echo "error: module directory '$MODULE' not found"
    exit 1
fi

# Validate module has go.mod
if [[ ! -f "$MODULE/go.mod" ]]; then
    echo "error: $MODULE/go.mod not found"
    exit 1
fi

TAG="${MODULE}/${VERSION}"

# Check tag doesn't already exist
if git tag -l "$TAG" | grep -q .; then
    echo "error: tag '$TAG' already exists"
    exit 1
fi

# Check working tree is clean
if ! git diff --quiet || ! git diff --cached --quiet; then
    echo "error: working tree has uncommitted changes"
    exit 1
fi

echo "→ Building $MODULE..."
go build "./${MODULE}/..."

echo "→ Testing $MODULE..."
go test "./${MODULE}/..." -count=1

echo "→ Creating tag $TAG..."
git tag -a "$TAG" -m "${MODULE} ${VERSION}"

echo "→ Pushing tag..."
git push origin "$TAG"

echo "✓ Released ${TAG}"
```

- [ ] **Step 2: Make script executable**

Run:
```bash
chmod +x scripts/release.sh
```

- [ ] **Step 3: Verify script rejects bad input**

Run:
```bash
./scripts/release.sh 2>&1 | grep -q "usage:" && echo "PASS: no args"
./scripts/release.sh go-common bad-version 2>&1 | grep -q "error: version" && echo "PASS: bad version"
./scripts/release.sh nonexistent v0.1.0 2>&1 | grep -q "error: module directory" && echo "PASS: bad module"
```
Expected: All three print PASS

- [ ] **Step 4: Commit**

```bash
git add scripts/release.sh
git commit -m "chore(release): add release.sh tag automation script (#29)"
```

---

### Task 3: CI Hygiene Check

**Files:**
- Modify: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: Clean go.mod invariant from Task 1
- Produces: CI job that fails if `replace` is re-introduced in published modules

- [ ] **Step 1: Add replace-directive check step to ci.yml**

Add a new job after the existing `build-test-lint` job in `.github/workflows/ci.yml`:

```yaml
  mod-hygiene:
    name: go.mod hygiene
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: check no replace in published modules
        run: |
          FAILED=0
          for mod in go-common go-auth go-middleware go-framework; do
            if grep -q '^replace' "$mod/go.mod"; then
              echo "::error file=${mod}/go.mod::contains replace directive — published modules must not use replace"
              FAILED=1
            fi
          done
          if [[ "$FAILED" -eq 1 ]]; then
            echo "Published modules must not contain replace directives."
            echo "Local development uses go.work workspace resolution."
            echo "See RELEASE.md for the release process."
            exit 1
          fi
          echo "✓ No replace directives in published modules"

      - name: check no v0.0.0 sibling requires
        run: |
          FAILED=0
          for mod in go-auth go-middleware go-framework; do
            if grep -q 'byx-darwin/go-tools/go-.* v0\.0\.0' "$mod/go.mod"; then
              echo "::error file=${mod}/go.mod::contains v0.0.0 sibling require — use real published versions"
              FAILED=1
            fi
          done
          if [[ "$FAILED" -eq 1 ]]; then
            echo "Sibling module requires must reference real published versions."
            exit 1
          fi
          echo "✓ No v0.0.0 sibling requires"
```

- [ ] **Step 2: Verify CI yaml is valid**

Run:
```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))" && echo "YAML valid"
```
Expected: `YAML valid`

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add go.mod hygiene check (no replace, no v0.0.0) (#29)"
```

---

### Task 4: RELEASE.md Documentation

**Files:**
- Create: `RELEASE.md`

**Interfaces:**
- Consumes: Decisions from design spec, release script from Task 2
- Produces: Developer-facing release documentation

- [ ] **Step 1: Create RELEASE.md**

```markdown
# Release Process

go-tools 包含四个独立版本化的 Go 模块，遵循 [Semantic Versioning](https://semver.org/)。

## Modules

| Module | Import Path | Dependencies |
|--------|------------|--------------|
| go-common | `github.com/byx-darwin/go-tools/go-common` | (none — bottom of DAG) |
| go-auth | `github.com/byx-darwin/go-tools/go-auth` | go-common |
| go-middleware | `github.com/byx-darwin/go-tools/go-middleware` | go-common, go-auth |
| go-framework | `github.com/byx-darwin/go-tools/go-framework` | go-common, go-auth |

## Version Policy

- **v0.x**: API 可能在不升 major 的情况下发生破坏性变更
- **v1.0.0**: 承诺 API 稳定，破坏性变更需升 major
- 各模块独立版本，不要求同步升级

## Tag Convention

```
<module>/v<major>.<minor>.<patch>[-prerelease]
```

Examples: `go-common/v0.1.0`, `go-auth/v1.2.3`, `go-framework/v2.0.0-rc.1`

## Local Development

本地开发使用 Go workspace 模式（默认）。`go.work` 的 `use` 指令让所有模块在本地互相解析，无需 `replace` 指令。

```bash
# 正常开发（workspace 模式，默认）
go build ./go-auth/...       # go-common 通过 go.work 解析

# 单模块模式（需要已发布的兄弟版本）
GOWORK=off go build ./go-auth/...  # 从 proxy 解析 go-common@v0.1.0
```

> ⚠️ 发布模块的 go.mod **禁止**包含 `replace` 指令。CI 会检查并阻断。

## Release Process

### Single Module Release

```bash
# 1. 确保依赖的兄弟模块已发布所需版本
#    例：go-auth v0.2.0 依赖 go-common v0.2.0

# 2. 更新 sibling require（如需要）
cd go-auth
go get github.com/byx-darwin/go-tools/go-common@v0.2.0
go mod tidy
cd ..
git add go-auth/go.mod go-auth/go.sum
git commit -m "chore(go-auth): bump go-common to v0.2.0"
git push

# 3. 打 tag 并推送
./scripts/release.sh go-auth v0.2.0
```

### Coordinated Multi-Module Release

当多个模块需要同时升级时，按依赖顺序发布：

```bash
# 1. go-common（无兄弟依赖）
./scripts/release.sh go-common v0.2.0

# 2. go-auth（依赖 go-common）
cd go-auth && go get github.com/byx-darwin/go-tools/go-common@v0.2.0 && go mod tidy && cd ..
git add go-auth/ && git commit -m "chore(go-auth): bump go-common to v0.2.0"
./scripts/release.sh go-auth v0.2.0

# 3. go-middleware + go-framework（依赖 go-common + go-auth，可并行）
cd go-middleware && go get github.com/byx-darwin/go-tools/go-common@v0.2.0 github.com/byx-darwin/go-tools/go-auth@v0.2.0 && go mod tidy && cd ..
git add go-middleware/ && git commit -m "chore(go-middleware): bump siblings to v0.2.0"
./scripts/release.sh go-middleware v0.2.0

cd go-framework && go get github.com/byx-darwin/go-tools/go-common@v0.2.0 github.com/byx-darwin/go-tools/go-auth@v0.2.0 && go mod tidy && cd ..
git add go-framework/ && git commit -m "chore(go-framework): bump siblings to v0.2.0"
./scripts/release.sh go-framework v0.2.0
```

### Release Script

`scripts/release.sh` 自动执行：
1. 验证参数格式（module 目录存在、version 格式正确、tag 不重复、工作区干净）
2. `go build` + `go test` 确保模块健康
3. 创建 annotated tag 并推送到 origin

### CI/CD

- **Tag push** 触发 `.github/workflows/release.yml`：build → test → 创建 GitHub Release
- **PR/push to main** 触发 `.github/workflows/ci.yml`：包含 go.mod 卫生检查（无 replace、无 v0.0.0）

## Consumer Usage

外部项目（如 ncgo 生成的项目）直接 require 即可：

```go
// go.mod
require (
    github.com/byx-darwin/go-tools/go-common v0.1.0
    github.com/byx-darwin/go-tools/go-auth v0.1.0
    github.com/byx-darwin/go-tools/go-framework v0.1.0
)
```

无需任何 `replace` 指令。

## History

| Version | Date | Notes |
|---------|------|-------|
| v0.1.0 | 2026-07-22 | 首次发布。三库拆分完成、安全审计修复完成。 |
```

- [ ] **Step 2: Commit**

```bash
git add RELEASE.md
git commit -m "docs: add RELEASE.md with versioning and release process (#29)"
```

---

### Task 5: External Consumer Verification Script

**Files:**
- Create: `scripts/verify-consumer.sh`

**Interfaces:**
- Consumes: Published module tags (post-release)
- Produces: Verification that external consumers can resolve all modules

- [ ] **Step 1: Create scripts/verify-consumer.sh**

```bash
#!/usr/bin/env bash
# verify-consumer.sh — Verify modules are resolvable from outside the workspace.
#
# Usage:
#   ./scripts/verify-consumer.sh [version]
#
# Default version: v0.1.0
#
# Creates a temporary module that imports all four libraries and verifies
# `go mod tidy` + `go build` succeed without any replace directives.
set -euo pipefail

VERSION="${1:-v0.1.0}"
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "→ Verifying external consumer resolution for version ${VERSION}..."
echo "  Temp dir: $TMPDIR"

cd "$TMPDIR"

# Initialize a fresh module (outside any workspace)
export GOWORK=off
go mod init verify-consumer

# Create main.go importing all four modules
cat > main.go << 'GOEOF'
package main

import (
	_ "github.com/byx-darwin/go-tools/go-common/cache"
	_ "github.com/byx-darwin/go-tools/go-auth/jwt"
	_ "github.com/byx-darwin/go-tools/go-middleware/redis"
	_ "github.com/byx-darwin/go-tools/go-framework/config"
)

func main() {}
GOEOF

# Add requires
go mod edit \
    -require="github.com/byx-darwin/go-tools/go-common@${VERSION}" \
    -require="github.com/byx-darwin/go-tools/go-auth@${VERSION}" \
    -require="github.com/byx-darwin/go-tools/go-middleware@${VERSION}" \
    -require="github.com/byx-darwin/go-tools/go-framework@${VERSION}"

echo "→ Running go mod tidy..."
go mod tidy

echo "→ Running go build..."
go build .

echo "✓ External consumer verification PASSED"
echo "  All four modules resolved from proxy without replace directives."
```

- [ ] **Step 2: Make script executable**

Run:
```bash
chmod +x scripts/verify-consumer.sh
```

- [ ] **Step 3: Commit**

```bash
git add scripts/verify-consumer.sh
git commit -m "chore(release): add external consumer verification script (#29)"
```

---

### Task 6: Final Validation

**Files:**
- None (validation only)

**Interfaces:**
- Consumes: All previous tasks
- Produces: Confidence that the workspace is release-ready

- [ ] **Step 1: Full workspace build**

Run:
```bash
go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...
```
Expected: Success (no output)

- [ ] **Step 2: Full workspace vet**

Run:
```bash
go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...
```
Expected: Success (no output)

- [ ] **Step 3: Full workspace tests**

Run:
```bash
go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1
```
Expected: All pass

- [ ] **Step 4: Lint per module**

Run:
```bash
for m in go-common go-auth go-middleware go-framework; do
  echo "=== $m ===" && golangci-lint run --timeout=5m ./$m/...
done
```
Expected: No issues

- [ ] **Step 5: gofmt check**

Run:
```bash
gofmt -l $(find . -name '*.go' -not -path '*/vendor/*' -not -path './.git/*')
```
Expected: No output (all files formatted)

- [ ] **Step 6: Verify go.mod hygiene locally**

Run:
```bash
for mod in go-common go-auth go-middleware go-framework; do
  if grep -q '^replace' "$mod/go.mod"; then
    echo "FAIL: $mod has replace"; exit 1
  fi
done
echo "✓ All published modules clean"
```
Expected: `✓ All published modules clean`

- [ ] **Step 7: Verify go mod tidy is stable**

Run:
```bash
cd go-auth && go mod tidy -diff && cd ..
cd go-middleware && go mod tidy -diff && cd ..
cd go-framework && go mod tidy -diff && cd ..
```
Expected: No diff output (go.sum already correct)
