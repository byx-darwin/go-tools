package log

import (
	"log/slog"
	"strings"
)

// Masker 敏感数据脱敏器。
type Masker struct {
	config MaskConfig
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
