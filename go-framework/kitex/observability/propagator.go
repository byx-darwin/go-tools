package observability

import (
	"context"

	"go.opentelemetry.io/otel/propagation"
)

// metadataSupplier 实现 propagation.TextMapCarrier，用于 metainfo map 的注入/提取。
type metadataSupplier struct {
	metadata map[string]string
}

// Get 获取指定 key 的值。
func (s *metadataSupplier) Get(key string) string {
	if s.metadata == nil {
		return ""
	}
	return s.metadata[key]
}

// Set 设置指定 key 的值。
func (s *metadataSupplier) Set(key, value string) {
	if s.metadata != nil {
		s.metadata[key] = value
	}
}

// Keys 返回所有 key。
func (s *metadataSupplier) Keys() []string {
	keys := make([]string, 0, len(s.metadata))
	for k := range s.metadata {
		keys = append(keys, k)
	}
	return keys
}

// Inject 将 TraceContext 注入到 metadata map。
func Inject(ctx context.Context, p propagation.TextMapPropagator, md map[string]string) {
	s := &metadataSupplier{metadata: md}
	p.Inject(ctx, s)
}

// Extract 从 metadata map 中提取 TraceContext。
func Extract(ctx context.Context, p propagation.TextMapPropagator, md map[string]string) context.Context {
	s := &metadataSupplier{metadata: md}
	return p.Extract(ctx, s)
}
