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
# Use a non-system temp dir (Go rejects go.mod in system temp roots like /var/folders)
WORKDIR="${HOME}/.cache/verify-consumer-$$"
mkdir -p "$WORKDIR"
trap 'rm -rf "$WORKDIR"' EXIT

echo "→ Verifying external consumer resolution for version ${VERSION}..."
echo "  Work dir: $WORKDIR"

cd "$WORKDIR"

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
