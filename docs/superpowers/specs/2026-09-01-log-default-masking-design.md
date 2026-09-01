# go-common/log 默认脱敏基线设计

- Issue: #61
- 日期: 2026-09-01
- 类型: bounded（有界变更，无需完整架构 spec）

## 背景

`log/config.go` 中 `MaskConfig.Enabled` 默认值为 `false`，脱敏字段列表 `MaskedFields` 需下游显式配置。下游服务若遗忘配置，`password`/`token`/`secret` 等敏感字段会明文写入日志。

## 决策

### 1. 默认脱敏字段列表

新增导出函数 `DefaultMaskedFields() []string`，返回：

```
password, passwd, secret, token, authorization, credential,
api_key, apikey, access_key, accesskey, secret_key, secretkey,
private_key, privatekey
```

**不包含** issue 中提到的裸词 `ak`/`sk`：`shouldMask` 是大小写不敏感的子串匹配，`ak`/`sk` 这类短词会误命中 `task_id`、`disk_usage`、`risk_score`、`bitmask` 等无关字段，因此排除。

### 2. `Enabled` 默认值

- `NewConfig()` 默认值新增 `Masking: MaskConfig{Enabled: true, MaskedFields: DefaultMaskedFields(), Mode: defaultMaskMode}`（新增常量 `defaultMaskMode = "full"`）。
- **零值行为不变**：直接使用 `Config{}` / `MaskConfig{}` 字面量（不经过 `NewConfig()`）仍是 `Enabled=false`，与仓库中 `Level`/`Format`/`Mode` 现有"默认值只在构造函数中生效"的惯例一致，不扩大改动面。
- `WithConfigMasking` 保持整体替换语义（与 `WithConfigFile`/`WithConfigCategories` 一致）；显式 `WithConfigMasking(MaskConfig{Enabled: false})` 即可关闭默认脱敏。

### 3. 破坏性影响

仅影响调用 `log.NewConfig()` 且未显式设置 `Masking` 的下游：原本明文的 `password`/`token`/`secret` 等字段会被脱敏为 `***`。仓库当前无 `go-common/CHANGELOG.md`，改动说明放入 PR 描述的 Breaking Change 段落。

## 测试计划

- `TestNewConfig_DefaultMaskingEnabled` — `NewConfig()` 默认 `Masking.Enabled=true` 且字段列表非空
- `TestDefaultMaskedFields_NoFalsePositive` — 默认列表不误命中 `task_id`/`disk_usage`/`risk_score`/`bitmask`
- `TestMasker_DefaultFields` — 用默认字段列表构造 `Masker`，验证敏感字段被脱敏
- `TestWithConfigMasking_ExplicitDisable` — 显式 `WithConfigMasking(MaskConfig{Enabled:false})` 可关闭默认脱敏
- 现有 `Config{}`/`MaskConfig{}` 零值相关测试保持不变（回归保护）

## 文档

更新 `MaskConfig`/`NewConfig` godoc，说明"经 `NewConfig()` 构造时默认启用脱敏 + 默认字段列表，零值/直接字面量构造不启用"。
