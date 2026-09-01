package log

import (
	"log/slog"
	"strings"
)

// Masker 敏感数据脱敏器。
type Masker struct {
	config MaskConfig
}

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

// NewMasker 创建脱敏器。
func NewMasker(cfg MaskConfig) *Masker {
	return &Masker{config: cfg}
}

// Mask 对日志属性进行脱敏处理。
func (m *Masker) Mask(attrs []slog.Attr) []slog.Attr {
	if !m.config.Enabled {
		return attrs
	}

	result := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		if m.shouldMask(attr.Key) {
			result[i] = slog.String(attr.Key, m.maskValue(attr.Value.String()))
		} else {
			result[i] = attr
		}
	}
	return result
}

// shouldMask 判断字段是否需要脱敏（大小写不敏感）。
func (m *Masker) shouldMask(key string) bool {
	key = strings.ToLower(key)
	for _, field := range m.config.MaskedFields {
		if strings.Contains(key, strings.ToLower(field)) {
			return true
		}
	}
	return false
}

// maskValue 根据模式脱敏值。
func (m *Masker) maskValue(value string) string {
	if m.config.Mode == "partial" {
		return m.partialMask(value)
	}
	return "***"
}

// partialMask 部分脱敏，保留首尾各 2 个字符。
func (m *Masker) partialMask(value string) string {
	if len(value) <= 4 {
		return "***"
	}
	return value[:2] + "***" + value[len(value)-2:]
}
