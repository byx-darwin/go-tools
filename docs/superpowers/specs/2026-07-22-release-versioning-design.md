# Release Versioning Design: Clean go.mod + go.work Overlay

> Issue: #29 (last item of tracking issue #34)
> Date: 2026-07-22
> Status: Approved
> Decision: Approach B — clean go.mod, go.work for local dev

## 1. Problem

`go-auth`, `go-middleware`, and `go-framework` carry `replace` directives and
`v0.0.0` require versions for sibling modules. The Go toolchain **ignores
`replace` directives in dependency go.mod files**, so external consumers (e.g.
ncgo-generated projects) attempting to resolve `go-framework` will try to fetch
`go-auth@v0.0.0` — which has no backing tag — and fail. This blocks D4
(independent versioning & release).

## 2. Decision Summary

| Item | Decision |
|------|----------|
| Release approach | **B: Clean go.mod + go.work overlay** |
| First version | **v0.1.0** for all four modules |
| Tag strategy | **Sequential**: go-common → go-auth → go-middleware + go-framework |
| Dev-time resolution | `go.work` `use` directives (already in place) |
| `example/` module | Keep `replace` (internal-only, never published) |

## 3. Target go.mod State

After this change, published modules' go.mod files contain **no `replace`
directives** and reference **real published versions** for siblings:

### go-common/go.mod (unchanged)
```
module github.com/byx-darwin/go-tools/go-common
// no sibling requires — bottom of DAG
```

### go-auth/go.mod
```diff
-require github.com/byx-darwin/go-tools/go-common v0.0.0
+require github.com/byx-darwin/go-tools/go-common v0.1.0

-replace github.com/byx-darwin/go-tools/go-common => ../go-common
```

### go-middleware/go.mod
```diff
-require github.com/byx-darwin/go-tools/go-auth v0.0.0
-require github.com/byx-darwin/go-tools/go-common v0.0.0
+require github.com/byx-darwin/go-tools/go-auth v0.1.0
+require github.com/byx-darwin/go-tools/go-common v0.1.0

-replace (
-    github.com/byx-darwin/go-tools/go-auth => ../go-auth
-    github.com/byx-darwin/go-tools/go-common => ../go-common
-)
```

### go-framework/go.mod
```diff
-require github.com/byx-darwin/go-tools/go-auth v0.0.0-00010101000000-000000000000
-require github.com/byx-darwin/go-tools/go-common v0.0.0
+require github.com/byx-darwin/go-tools/go-auth v0.1.0
+require github.com/byx-darwin/go-tools/go-common v0.1.0

-replace (
-    github.com/byx-darwin/go-tools/go-auth => ../go-auth
-    github.com/byx-darwin/go-tools/go-common => ../go-common
-)
```

### example/go.mod (NOT published — keep replace for convenience)
```
// Keep replace directives. example/ is in go.work `use` and is never tagged.
```

## 4. Bootstrap Sequence (First Release)

Because go-auth requires a resolvable go-common tag, the first release must be
sequential:

```
Step 1: Remove replace from go-auth/go.mod, set go-common require to v0.1.0
        (temporarily use go.work to build/test — workspace resolves locally)
Step 2: git tag go-common/v0.1.0 && git push origin go-common/v0.1.0
        → wait for proxy.golang.org to index (or GOPROXY=direct)
Step 3: Remove replace from go-auth/go.mod (already done in step 1)
        git tag go-auth/v0.1.0 && git push origin go-auth/v0.1.0
Step 4: Remove replace from go-middleware + go-framework, set requires to v0.1.0
        git tag go-middleware/v0.1.0 && git push
        git tag go-framework/v0.1.0 && git push
```

**Key constraint:** All four tags point to the SAME commit (or sequential
commits on main). The go.mod files at each tag must reference only
already-tagged sibling versions.

### Practical Implementation

Since all tags should point to the same final commit (where all go.mod files
are clean), the actual sequence is:

1. Make ONE commit that cleans all go.mod files (removes replace, sets v0.1.0),
   run `go mod tidy` in each affected module to update go.sum
2. Push to main
3. Tag sequentially from that commit:
   ```bash
   git tag go-common/v0.1.0 <commit>
   git push origin go-common/v0.1.0
   # wait for proxy or use GOPROXY=direct for verification
   git tag go-auth/v0.1.0 <commit>
   git push origin go-auth/v0.1.0
   git tag go-middleware/v0.1.0 <commit>
   git push origin go-middleware/v0.1.0
   git tag go-framework/v0.1.0 <commit>
   git push origin go-framework/v0.1.0
   ```

This works because at the tagged commit:
- `go-common` has no sibling requires → immediately resolvable
- `go-auth` requires `go-common@v0.1.0` → resolvable once go-common tag exists
- `go-middleware`/`go-framework` require both → resolvable once both tags exist

The proxy fetches modules lazily, so as long as tags exist before a consumer
tries to resolve the full dependency graph, all is well.

## 5. Release Script

Create `scripts/release.sh` to automate future releases:

```bash
#!/usr/bin/env bash
# Usage: ./scripts/release.sh <module> <version>
# Example: ./scripts/release.sh go-common v0.2.0
set -euo pipefail

MODULE="$1"
VERSION="$2"
TAG="${MODULE}/${VERSION}"

# Pre-flight checks
[[ -d "$MODULE" ]] || { echo "error: module dir $MODULE not found"; exit 1; }
git diff --quiet || { echo "error: working tree dirty"; exit 1; }
git tag -l "$TAG" | grep -q . && { echo "error: tag $TAG already exists"; exit 1; }

# Verify module builds and tests pass
go build "./${MODULE}/..."
go test "./${MODULE}/..." -count=1

# Create and push tag
git tag -a "$TAG" -m "${MODULE} ${VERSION}"
git push origin "$TAG"
echo "✓ Tagged and pushed ${TAG}"
```

For **subsequent releases** that bump sibling versions:
```bash
# Example: releasing go-auth v0.2.0 which depends on go-common v0.2.0
cd go-auth
go get github.com/byx-darwin/go-tools/go-common@v0.2.0
go mod tidy
cd ..
git add go-auth/go.mod go-auth/go.sum
git commit -m "chore(go-auth): bump go-common to v0.2.0"
./scripts/release.sh go-auth v0.2.0
```

## 6. CI Enhancement

Add a **go.mod hygiene check** to `ci.yml` to prevent `replace` directives
from being re-introduced in published modules:

```yaml
- name: check no replace in published modules
  run: |
    for mod in go-common go-auth go-middleware go-framework; do
      if grep -q '^replace' "$mod/go.mod"; then
        echo "::error::$mod/go.mod contains replace directive"
        exit 1
      fi
    done
```

This runs on every PR and push to main, ensuring the clean go.mod invariant
is maintained.

## 7. Documentation

Create `RELEASE.md` at repo root documenting:

1. **Module structure** — 4 independently versioned modules, DAG topology
2. **Version policy** — SemVer, v0.x allows breaking changes, v1.0.0 = stable API
3. **Release process** — step-by-step for single-module and coordinated releases
4. **Tag convention** — `<module>/v<major>.<minor>.<patch>`
5. **Consumer usage** — how ncgo projects import these modules
6. **Bootstrap history** — note that v0.1.0 was the initial coordinated release

## 8. Verification

After tagging, verify with an external consumer test:

```bash
# Create a temp module outside the workspace
TMPDIR=$(mktemp -d)
cd "$TMPDIR"
go mod init verify-consumer
cat > main.go << 'EOF'
package main

import (
    _ "github.com/byx-darwin/go-tools/go-common/cache"
    _ "github.com/byx-darwin/go-tools/go-auth/jwt"
    _ "github.com/byx-darwin/go-tools/go-middleware/redis"
    _ "github.com/byx-darwin/go-tools/go-framework/hertz/server"
)

func main() {}
EOF
go mod tidy  # must succeed without replace
go build .   # must compile
```

Success criteria: `go mod tidy` resolves all modules from proxy without errors.

## 9. Development Workflow (Post-Change)

With `replace` removed, local development relies on `go.work`:

```bash
# Normal development (workspace mode — default)
go build ./go-auth/...    # resolves go-common via go.work `use`

# Single-module mode (GOWORK=off) — requires published sibling versions
GOWORK=off go build ./go-auth/...  # resolves go-common@v0.1.0 from proxy
```

The `go.work` file already lists all modules in `use`, so no changes needed
there. Developers must always build from the workspace root (or with go.work
active), which is the existing workflow.

## 10. Risk & Mitigation

| Risk | Mitigation |
|------|-----------|
| Proxy index delay after first tag | Use `GOPROXY=direct` for verification; wait ~5min for proxy.golang.org |
| Developer builds with GOWORK=off before tags exist | Document: workspace mode is required until v0.1.0 tags are pushed |
| Accidental replace re-introduction | CI check (§6) blocks PRs that add replace to published modules |
| example/ module breaks | example/ keeps replace; it's in go.work `use` so both paths work |

## 11. Scope Boundaries

**In scope:**
- Remove `replace` from go-auth, go-middleware, go-framework go.mod
- Update sibling `require` to v0.1.0
- Create `scripts/release.sh`
- Add CI hygiene check
- Write `RELEASE.md`
- Verification script/test

**Out of scope:**
- `example/` module cleanup (keeps replace)
- Actual tag creation (done separately after PR merge)
- ncgo template updates (separate issue)
- go.sum changes (handled by `go mod tidy`)
