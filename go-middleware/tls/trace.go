package tls

import (
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// instrumentationName OTel instrumentation 标识。
const instrumentationName = "github.com/byx-darwin/go-tools/go-middleware/tls"

// endSpan 出错时记录错误并设置 span 状态，最终结束 span。
func endSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

// spanLinks 把待关联的 SpanContext 按 SpanID 去重后转换为 trace.Link，
// 用于关联触发本次 flush 的各 SendLog 调用方 span。
func spanLinks(scs []trace.SpanContext) []trace.Link {
	if len(scs) == 0 {
		return nil
	}
	seen := make(map[trace.SpanID]struct{}, len(scs))
	links := make([]trace.Link, 0, len(scs))
	for _, sc := range scs {
		if _, ok := seen[sc.SpanID()]; ok {
			continue
		}
		seen[sc.SpanID()] = struct{}{}
		links = append(links, trace.Link{SpanContext: sc})
	}
	return links
}
