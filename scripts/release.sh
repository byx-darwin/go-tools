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
