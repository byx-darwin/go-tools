# 设计文档：删除 HTTPAuth.TeaKey 孤儿配置字段

- **Issue**: #46
- **日期**: 2026-07-22
- **决策**: 方案 B — 删除字段
- **关联**: #25 (TEA→AES-GCM) · PR #45 · #34 (审计修复路线图)

## 背景

`go-common` 的 TEA 加密已在 #25（PR #45）中彻底移除并迁移到 AES-GCM。
`go-framework/config/hertz/config.go` 中残留一个指向 TEA 的配置字段：

```go
type HTTPAuth struct {
    Enable bool   `json:"enable"  yaml:"enable"`
    AK     string `json:"ak"  yaml:"ak"`
    SK     string `json:"sk"  yaml:"sk"`
    TeaKey string `json:"tea_key"  yaml:"tea_key"`   // ← 孤儿字段
}
```

### 代码分析结论

| 检查项 | 结果 |
|--------|------|
| 全仓库 `TeaKey` 功能性读取 | **零** — 无任何代码用它做加密 |
| AK/SK 鉴权中间件 (`middleware/auth.go`) | 纯 HMAC-SHA256 签名验证，不使用对称加密密钥 |
| 唯一其他引用 | `example/handler/config_handler.go:204` 脱敏清单中的字符串 `"tea_key"` |
| YAML 反序列化 | `config.LoadYAML` 使用 `yaml.Unmarshal`（非 strict 模式），忽略未知字段 |

### 决策依据

选择 **方案 B（删除）** 而非改名复用（A）或保留重命名（C），原因：

1. AK/SK 鉴权是纯 HMAC 签名机制，不需要对称加密密钥，没有"复用为 AESKey"的场景
2. 保留一个无功能的字段只会造成语义误导
3. `yaml.Unmarshal` 不使用 `KnownFields(true)`，旧配置中的 `tea_key:` 会被安全忽略，**不会产生运行时错误**

## 变更范围

### 本仓库（go-tools）

| 文件 | 变更 |
|------|------|
| `go-framework/config/hertz/config.go:35` | 删除 `TeaKey` 字段 |
| `go-framework/config/hertz/config_test.go:32,44` | 移除 `TeaKey` 相关测试断言 |
| `example/handler/config_handler.go:204` | 从脱敏清单中移除 `"tea_key"` |
| `example/config.yaml:71` | 删除 `tea_key: "0123456789abcdef"` 行 |

### 不在本次范围（follow-up）

| 项目 | 说明 |
|------|------|
| ncgo 脚手架模板 | ncgo 是独立仓库，需单独更新配置模板与示例 YAML |
| 迁移说明 | 旧 `tea_key` 配置无需手动清理（yaml 忽略未知字段），但建议在 ncgo 更新时附带说明 |

## 向后兼容性

- **YAML 配置兼容**: `yaml.Unmarshal` 忽略未知字段，已部署服务的 `config.yaml` 中残留 `tea_key:` 不会报错
- **Go 源码兼容**: 这是 source-breaking change — 任何直接引用 `HTTPAuth.TeaKey` 的 Go 代码会编译失败。已确认本仓库内无此类引用
- **JSON 配置兼容**: 同理，`json.Unmarshal` 忽略未知字段

## 验证标准

- `go build ./go-framework/...` 通过
- `go vet ./go-framework/...` 通过
- `golangci-lint run --timeout=5m ./go-framework/...` 通过
- `go test ./go-framework/... -count=1` 通过
- 全仓库 `grep -r "TeaKey\|tea_key"` 无残留（设计文档除外）
