package log

import (
	"log/slog"

	"github.com/samber/oops"
)

// ErrorAttrs 从 oops 错误中提取结构化日志属性。
// 如果错误不是 oops 错误，返回空切片。
func ErrorAttrs(err error) []any {
	if err == nil {
		return nil
	}

	oopsErr, ok := oops.AsOops(err)
	if !ok {
		return nil
	}

	var attrs []any

	if code := oopsErr.Code(); code != nil {
		attrs = append(attrs, slog.Any("error.code", code))
	}

	if domain := oopsErr.Domain(); domain != "" {
		attrs = append(attrs, slog.String("error.domain", domain))
	}

	if hint := oopsErr.Hint(); hint != "" {
		attrs = append(attrs, slog.String("error.hint", hint))
	}

	if public := oopsErr.Public(); public != "" {
		attrs = append(attrs, slog.String("error.public", public))
	}

	return attrs
}
