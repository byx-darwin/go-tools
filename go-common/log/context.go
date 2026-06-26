package log

import "context"

// contextKey 是 context 值的键类型，避免与其他包冲突。
type contextKey string

// 预定义 context key 常量。
const (
	// ContextKeyRequestID 请求 ID 的 context key。
	ContextKeyRequestID = "request_id"

	// ContextKeyTraceID Trace ID 的 context key。
	ContextKeyTraceID = "trace_id"

	// ContextKeySpanID Span ID 的 context key。
	ContextKeySpanID = "span_id"
)

// WithContextValue 向 context 添加字符串值。
func WithContextValue(ctx context.Context, key, value string) context.Context {
	return context.WithValue(ctx, contextKey(key), value)
}

// ContextValue 从 context 获取字符串值，不存在返回空字符串。
func ContextValue(ctx context.Context, key string) string {
	if v, ok := ctx.Value(contextKey(key)).(string); ok {
		return v
	}
	return ""
}

// WithRequestID 向 context 注入请求 ID。
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return WithContextValue(ctx, ContextKeyRequestID, requestID)
}

// RequestIDFromContext 从 context 获取请求 ID。
func RequestIDFromContext(ctx context.Context) string {
	return ContextValue(ctx, ContextKeyRequestID)
}
