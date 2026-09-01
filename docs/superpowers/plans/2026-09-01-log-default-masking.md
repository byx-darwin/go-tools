# go-common/log 默认脱敏基线 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `go-common/log.NewConfig()` 默认启用脱敏并使用一份安全的默认字段列表，降低下游遗忘配置导致敏感信息明文落盘的风险。

**Architecture:** 在 `mask.go` 新增导出函数 `DefaultMaskedFields()` 提供默认字段列表；在 `config.go` 的 `NewConfig()` 默认值中接入该列表并把 `Masking.Enabled` 置为 `true`；`Config{}`/`MaskConfig{}` 裸零值不受影响，`WithConfigMasking` 保持整体替换语义不变。

**Tech Stack:** Go 1.25（workspace `go-common` module），`log/slog`，`testify/require`。

**Spec:** `docs/superpowers/specs/2026-09-01-log-default-masking-design.md`

## Global Constraints

- 仅修改 `go-common/log` 包内文件，不跨模块。
- 不改变 `Config{}` / `MaskConfig{}` 裸字面量的零值行为（`Enabled` 仍为 `false`）——只有经 `NewConfig()` 构造时才应用新默认值。
- `shouldMask`（`mask.go:36-44`）子串匹配逻辑本身不变；默认字段列表禁止包含 `ak`/`sk` 等会误命中 `task_id`/`disk_usage`/`risk_score`/`bitmask` 的短词。
- 所有导出符号必须有以符号名开头的 godoc 注释（项目 lint 规则 `.claude/rules/go.md` §8.3）。
- 新增函数不得引入未处理的 error 返回值（本任务不涉及）。
- PR 描述必须包含 Breaking Change 段落说明：`NewConfig()` 默认值变更（无 `go-common/CHANGELOG.md` 可写）。

---

### Task 1: `DefaultMaskedFields()` 与默认字段列表测试

**Files:**
- Modify: `go-common/log/mask.go`
- Test: `go-common/log/mask_test.go`

**Interfaces:**
- Produces: `func DefaultMaskedFields() []string` — 返回默认脱敏字段名列表（每次调用返回新的 slice，避免调用方修改共享底层数组）。

- [ ] **Step 1: 写失败测试 — 默认列表内容**

在 `go-common/log/mask_test.go` 追加：

```go
func TestDefaultMaskedFields_ContainsExpected(t *testing.T) {
	fields := log.DefaultMaskedFields()
	for _, want := range []string{
		"password", "passwd", "secret", "token", "authorization",
		"credential", "api_key", "apikey", "access_key", "accesskey",
		"secret_key", "secretkey", "private_key", "privatekey",
	} {
		require.Contains(t, fields, want)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./go-common/log/... -run TestDefaultMaskedFields_ContainsExpected -v`
Expected: FAIL，`undefined: log.DefaultMaskedFields`

- [ ] **Step 3: 写失败测试 — 无误命中**

继续在 `mask_test.go` 追加：

```go
func TestDefaultMaskedFields_NoFalsePositive(t *testing.T) {
	cfg := log.MaskConfig{
		Enabled:      true,
		MaskedFields: log.DefaultMaskedFields(),
		Mode:         "full",
	}
	masker := log.NewMasker(cfg)
	attrs := []slog.Attr{
		slog.String("task_id", "t-1"),
		slog.String("disk_usage", "42%"),
		slog.String("risk_score", "0.1"),
		slog.String("bitmask", "0xFF"),
	}
	masked := masker.Mask(attrs)
	for i, attr := range attrs {
		require.Equal(t, attr.Value.String(), masked[i].Value.String(), "field %s should not be masked", attr.Key)
	}
}
```

- [ ] **Step 4: 写失败测试 — 默认字段生效脱敏**

继续追加：

```go
func TestMasker_DefaultFields(t *testing.T) {
	cfg := log.MaskConfig{
		Enabled:      true,
		MaskedFields: log.DefaultMaskedFields(),
		Mode:         "full",
	}
	masker := log.NewMasker(cfg)
	attrs := []slog.Attr{
		slog.String("password", "secret123"),
		slog.String("token", "abc.def.ghi"),
		slog.String("secret", "topsecret"),
		slog.String("api_key", "sk-live-xxx"),
	}
	masked := masker.Mask(attrs)
	for i, attr := range masked {
		require.Equal(t, "***", attr.Value.String(), "field %s should be masked", attrs[i].Key)
	}
}
```

- [ ] **Step 5: 确认三个新测试均失败**

Run: `go test ./go-common/log/... -run 'TestDefaultMaskedFields|TestMasker_DefaultFields' -v`
Expected: 全部 FAIL（`DefaultMaskedFields` 未定义）

- [ ] **Step 6: 实现 `DefaultMaskedFields()`**

在 `go-common/log/mask.go` 中，`NewMasker` 函数之前新增：

```go
// defaultMaskedFields 默认脱敏字段基线，仅使用完整词避免误命中
// task_id / disk_usage / risk_score / bitmask 等无关字段。
var defaultMaskedFields = []string{
	"password", "passwd", "secret", "token", "authorization",
	"credential", "api_key", "apikey", "access_key", "accesskey",
	"secret_key", "secretkey", "private_key", "privatekey",
}

// DefaultMaskedFields 返回默认脱敏字段名列表的副本。
// 用于 NewConfig 的开箱即用脱敏基线，也可供调用方在此基础上追加自定义字段。
func DefaultMaskedFields() []string {
	fields := make([]string, len(defaultMaskedFields))
	copy(fields, defaultMaskedFields)
	return fields
}
```

- [ ] **Step 7: 运行确认通过**

Run: `go test ./go-common/log/... -run 'TestDefaultMaskedFields|TestMasker_DefaultFields' -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add go-common/log/mask.go go-common/log/mask_test.go
git commit -m "feat(go-common/log): add DefaultMaskedFields baseline"
```

---

### Task 2: `NewConfig()` 默认启用脱敏

**Files:**
- Modify: `go-common/log/config.go`
- Modify: `go-common/log/config_test.go`

**Interfaces:**
- Consumes: `log.DefaultMaskedFields() []string`（Task 1 产出）
- Produces: `NewConfig()` 返回值中 `Config.Masking` 默认值变为 `MaskConfig{Enabled: true, MaskedFields: DefaultMaskedFields(), Mode: defaultMaskMode}`；新增包级常量 `defaultMaskMode = "full"`。

- [ ] **Step 1: 更新既有测试以反映新默认行为（RED）**

`go-common/log/config_test.go` 中 `TestConfig_Defaults`（第 10-18 行）当前断言 `require.False(t, cfg.Masking.Enabled)`，与新设计冲突，替换为：

```go
func TestConfig_Defaults(t *testing.T) {
	cfg := log.NewConfig()
	require.Equal(t, "info", cfg.Level)
	require.Equal(t, "json", cfg.Format)
	require.Equal(t, "console", cfg.Mode)
	require.False(t, cfg.AddSource)
	require.Empty(t, cfg.Categories)
	require.True(t, cfg.Masking.Enabled)
	require.NotEmpty(t, cfg.Masking.MaskedFields)
	require.Equal(t, "full", cfg.Masking.Mode)
}
```

- [ ] **Step 2: 追加新测试 — 默认字段来自 `DefaultMaskedFields`**

在 `config_test.go` 追加：

```go
func TestNewConfig_DefaultMaskingEnabled(t *testing.T) {
	cfg := log.NewConfig()
	require.True(t, cfg.Masking.Enabled)
	require.Equal(t, log.DefaultMaskedFields(), cfg.Masking.MaskedFields)
	require.Equal(t, "full", cfg.Masking.Mode)
}
```

- [ ] **Step 3: 追加新测试 — 显式禁用仍生效（向后兼容路径）**

继续追加：

```go
func TestWithConfigMasking_ExplicitDisable(t *testing.T) {
	cfg := log.NewConfig(
		log.WithConfigMasking(log.MaskConfig{Enabled: false}),
	)
	require.False(t, cfg.Masking.Enabled)
	require.Empty(t, cfg.Masking.MaskedFields)
}
```

- [ ] **Step 4: 追加零值回归测试 — 裸字面量不受影响**

继续追加：

```go
func TestMaskConfig_ZeroValueUnaffected(t *testing.T) {
	var cfg log.Config
	require.False(t, cfg.Masking.Enabled)
	require.Empty(t, cfg.Masking.MaskedFields)
}
```

- [ ] **Step 5: 运行确认失败**

Run: `go test ./go-common/log/... -run 'TestConfig_Defaults|TestNewConfig_DefaultMaskingEnabled|TestWithConfigMasking_ExplicitDisable|TestMaskConfig_ZeroValueUnaffected' -v`
Expected: `TestConfig_Defaults`、`TestNewConfig_DefaultMaskingEnabled` FAIL（当前 `Masking.Enabled` 仍为 `false`）；`TestWithConfigMasking_ExplicitDisable`、`TestMaskConfig_ZeroValueUnaffected` PASS（零值路径未改动）

- [ ] **Step 6: 实现默认值变更**

在 `go-common/log/config.go`：

1. 在 `Config` 默认值常量块（第 230-239 行）追加：

```go
const (
	defaultConfigLevel  = "info"
	defaultConfigFormat = "json"
	defaultConfigMode   = "console"

	defaultFileMaxSize    = 100
	defaultFileMaxBackups = 7
	defaultFileMaxAge     = 30

	// defaultMaskMode 默认脱敏模式（NewConfig 使用）。
	defaultMaskMode = "full"
)
```

2. 修改 `NewConfig()`（第 144-155 行）：

```go
// NewConfig 创建 Config，支持 Options 配置。
//
// 默认配置：
//   - level: "info"
//   - format: "json"
//   - mode: "console"
//   - masking: 启用，使用 DefaultMaskedFields() 默认字段列表，mode "full"
//
// 注意：仅通过 NewConfig 构造时应用上述默认值；直接使用 Config{} 零值字面量
// 不会自动启用脱敏（Masking.Enabled 保持 false），与 Level/Format/Mode 的
// 现有约定一致。
func NewConfig(opts ...ConfigOption) Config {
	cfg := Config{
		Level:  defaultConfigLevel,
		Format: defaultConfigFormat,
		Mode:   defaultConfigMode,
		File:   NewFileConfig(),
		Masking: MaskConfig{
			Enabled:      true,
			MaskedFields: DefaultMaskedFields(),
			Mode:         defaultMaskMode,
		},
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}
```

3. 更新 `MaskConfig` 结构体上方注释（第 66 行）：

```go
// MaskConfig 敏感数据脱敏配置。
//
// 经 NewConfig() 构造时默认 Enabled=true 并附带 DefaultMaskedFields()
// 基线字段；直接使用 MaskConfig{} 零值字面量则默认关闭（Enabled=false）。
// WithConfigMasking 会整体替换该结构体，若需在默认基础上追加自定义字段，
// 调用方应显式传入 MaskConfig{Enabled: true, MaskedFields: append(DefaultMaskedFields(), "custom_field"), Mode: "full"}。
type MaskConfig struct {
```

4. 更新 `WithConfigMasking` 注释（第 131 行），补充整体替换语义提示：

```go
// WithConfigMasking 设置脱敏配置（整体替换，与 WithConfigFile 一致）。
// 传入 MaskConfig{Enabled: false} 可显式关闭 NewConfig 的默认脱敏基线。
func WithConfigMasking(masking MaskConfig) ConfigOption {
```

- [ ] **Step 7: 运行确认全部通过**

Run: `go test ./go-common/log/... -v`
Expected: 全部 PASS（含既有 `TestConfig_WithMasking`、`TestMaskConfig_Fields` 等不受影响的用例）

- [ ] **Step 8: Commit**

```bash
git add go-common/log/config.go go-common/log/config_test.go
git commit -m "feat(go-common/log): enable default masking baseline in NewConfig"
```

---

### Task 3: 全量校验与 PR 说明素材

**Files:**
- Test: 全量 `go-common` 模块

- [ ] **Step 1: 全量测试**

Run: `go test ./go-common/... -count=1`
Expected: PASS

- [ ] **Step 2: 构建与静态检查**

Run:
```bash
go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...
go vet ./go-common/...
gofmt -l go-common/log
golangci-lint run --timeout=5m ./go-common/...
```
Expected: 均无错误输出（`gofmt -l` 无输出表示已格式化）

- [ ] **Step 3: 起草 PR Breaking Change 说明（供 Phase 3 `gf-pr-create` 使用，非提交内容）**

```markdown
## Breaking Change

`go-common/log.NewConfig()` 的默认值变更：`Masking.Enabled` 由 `false` 改为 `true`，
并默认附带 `DefaultMaskedFields()` 基线字段（password/token/secret/authorization/
credential/api_key/access_key/secret_key/private_key 等长词，不含 ak/sk 短词以避免
误命中 task_id/disk_usage 等字段）。

**影响范围**：仅影响调用 `log.NewConfig()` 且未显式设置 `Masking` 的下游——这些
调用方原本明文输出的敏感字段现在会被脱敏为 `***`。

**不受影响**：直接使用 `log.Config{}` / `log.MaskConfig{}` 零值字面量构造的调用方
行为不变（`Enabled` 仍为 `false`），与 `Level`/`Format`/`Mode` 现有的
"默认值仅在构造函数中生效"惯例一致。

**如何保留旧行为**：`log.NewConfig(log.WithConfigMasking(log.MaskConfig{Enabled: false}))`。

本仓库 `go-common` 无 `CHANGELOG.md`，此说明作为该变更的唯一记录，请在合并时保留于
PR 描述中。
```

- [ ] **Step 4: Commit（若有格式化改动）**

```bash
git add -A
git commit -m "chore(go-common/log): gofmt after masking default change" --allow-empty
```
（若 Step 2 未产生任何文件改动，跳过本次 commit）
