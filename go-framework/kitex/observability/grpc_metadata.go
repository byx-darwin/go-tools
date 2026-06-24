package observability

import (
	"context"

	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// grpcMetadataSupplier 实现 propagation.TextMapCarrier，用于 gRPC metadata 的注入/提取。
type grpcMetadataSupplier struct {
	metadata *metadata.MD
}

// Get 从 gRPC metadata 中获取指定 key 的第一个值。
func (s *grpcMetadataSupplier) Get(key string) string {
	if s.metadata == nil {
		return ""
	}
	values := (*s.metadata)[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// Set 向 gRPC metadata 设置 key/value。
func (s *grpcMetadataSupplier) Set(key, value string) {
	if s.metadata != nil {
		(*s.metadata)[key] = []string{value}
	}
}

// Keys 返回 gRPC metadata 中的所有 key。
func (s *grpcMetadataSupplier) Keys() []string {
	if s.metadata == nil {
		return nil
	}
	keys := make([]string, 0, len(*s.metadata))
	for k := range *s.metadata {
		keys = append(keys, k)
	}
	return keys
}

// injectGRPCMetadata 将 trace context 注入到 gRPC outgoing metadata。
func injectGRPCMetadata(ctx context.Context, md metadata.MD) context.Context {
	propagator := otel.GetTextMapPropagator()
	propagator.Inject(ctx, &grpcMetadataSupplier{metadata: &md})
	return ctx
}

// extractGRPCMetadata 从 gRPC incoming metadata 提取 trace context。
func extractGRPCMetadata(ctx context.Context, md metadata.MD) context.Context {
	propagator := otel.GetTextMapPropagator()
	return propagator.Extract(ctx, &grpcMetadataSupplier{metadata: &md})
}

// extractPeerServiceAttributesFromGRPCMetadata 从 gRPC metadata 提取对端服务属性。
func extractPeerServiceAttributesFromGRPCMetadata(md metadata.MD) []attribute.KeyValue {
	var attrs []attribute.KeyValue
	s := &grpcMetadataSupplier{metadata: &md}

	if v := s.Get(string(semconv.ServiceNameKey)); v != "" {
		attrs = append(attrs, semconv.PeerServiceKey.String(v))
	}
	if v := s.Get(string(semconv.ServiceNamespaceKey)); v != "" {
		attrs = append(attrs, PeerServiceNamespaceKey.String(v))
	}
	if v := s.Get(string(semconv.DeploymentEnvironmentKey)); v != "" {
		attrs = append(attrs, PeerDeploymentEnvironmentKey.String(v))
	}

	return attrs
}
