package kafka

import (
	"context"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// instrumentationName OTel instrumentation 标识。
const instrumentationName = "github.com/byx-darwin/go-tools/go-middleware/kafka"

// headerCarrier 实现 propagation.TextMapCarrier，用于把 trace 上下文注入/提取到
// kafka.Message.Headers。
type headerCarrier struct {
	headers *[]kafka.Header
}

func (c headerCarrier) Get(key string) string {
	for _, h := range *c.headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c headerCarrier) Set(key, value string) {
	for i, h := range *c.headers {
		if h.Key == key {
			(*c.headers)[i].Value = []byte(value)
			return
		}
	}
	*c.headers = append(*c.headers, kafka.Header{Key: key, Value: []byte(value)})
}

func (c headerCarrier) Keys() []string {
	keys := make([]string, len(*c.headers))
	for i, h := range *c.headers {
		keys[i] = h.Key
	}
	return keys
}

// injectTraceContext 把 ctx 中的 span 上下文注入消息 Headers，供消费端提取关联。
func injectTraceContext(ctx context.Context, headers *[]kafka.Header) {
	otel.GetTextMapPropagator().Inject(ctx, headerCarrier{headers: headers})
}

// extractTraceContext 从消息 Headers 里提取生产端的 trace 上下文。
func extractTraceContext(headers []kafka.Header) context.Context {
	return otel.GetTextMapPropagator().Extract(context.Background(), headerCarrier{headers: &headers})
}

// endSpan 出错时记录错误并设置 span 状态，最终结束 span。
func endSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
