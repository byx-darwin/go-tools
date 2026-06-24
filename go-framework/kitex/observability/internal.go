package observability

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// traceCarrier 在 stats.Tracer 的 Start/Finish 和 Middleware 之间传递 span 和 tracer。
type traceCarrier struct {
	tracer trace.Tracer
	span   trace.Span
}

// SetTracer 设置 tracer。
func (tc *traceCarrier) SetTracer(t trace.Tracer) {
	tc.tracer = t
}

// Tracer 返回 tracer。
func (tc *traceCarrier) Tracer() trace.Tracer {
	return tc.tracer
}

// SetSpan 设置 span。
func (tc *traceCarrier) SetSpan(s trace.Span) {
	tc.span = s
}

// Span 返回 span。
func (tc *traceCarrier) Span() trace.Span {
	return tc.span
}

type traceCarrierKey struct{}

// withTraceCarrier 将 TraceCarrier 注入 context。
func withTraceCarrier(ctx context.Context, tc *traceCarrier) context.Context {
	return context.WithValue(ctx, traceCarrierKey{}, tc)
}

// traceCarrierFromContext 从 context 提取 TraceCarrier。
func traceCarrierFromContext(ctx context.Context) *traceCarrier {
	tc, ok := ctx.Value(traceCarrierKey{}).(*traceCarrier)
	if !ok {
		return nil
	}
	return tc
}
