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
