package observability

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// rpcMetricsAttributes 定义 RPC metrics 应携带的属性 key（对齐 OTel RPC semantic conventions）。
var rpcMetricsAttributes = []attribute.Key{
	semconv.RPCServiceKey,
	semconv.RPCSystemKey,
	semconv.RPCMethodKey,
	attribute.Key("net.peer.name"),
	attribute.Key("net.transport"),
}

// peerMetricsAttributes 定义对端服务的属性 key。
var peerMetricsAttributes = []attribute.Key{
	semconv.PeerServiceKey,
	PeerServiceNamespaceKey,
	PeerDeploymentEnvironmentKey,
	RequestProtocolKey,
	SourceOperationKey,
}

// metricResourceAttributes 定义 resource 级别的 metrics 属性 key。
var metricResourceAttributes = []attribute.Key{
	semconv.ServiceNameKey,
	semconv.ServiceNamespaceKey,
	semconv.DeploymentEnvironmentKey,
	semconv.ServiceInstanceIDKey,
	semconv.ServiceVersionKey,
	semconv.TelemetrySDKLanguageKey,
	semconv.TelemetrySDKVersionKey,
	semconv.ProcessPIDKey,
	semconv.HostNameKey,
	semconv.HostIDKey,
}

// extractMetricsAttributes 从 span 中提取用于 metrics 记录的属性列表。
//
// 提取规则：
//   - span attributes 中匹配 rpcMetricsAttributes 或 peerMetricsAttributes 的 key
//   - span resource attributes 中匹配 metricResourceAttributes 的 key
//   - span status code
func extractMetricsAttributes(span oteltrace.Span) []attribute.KeyValue {
	var attrs []attribute.KeyValue
	readOnlySpan, ok := span.(trace.ReadOnlySpan)
	if !ok {
		return attrs
	}

	for _, attr := range readOnlySpan.Attributes() {
		if matchAttributeKey(attr.Key, rpcMetricsAttributes) {
			attrs = append(attrs, attr)
		}
		if matchAttributeKey(attr.Key, peerMetricsAttributes) {
			attrs = append(attrs, attr)
		}
	}

	for _, attr := range readOnlySpan.Resource().Attributes() {
		if matchAttributeKey(attr.Key, metricResourceAttributes) {
			attrs = append(attrs, attr)
		}
	}

	attrs = append(attrs, StatusKey.String(readOnlySpan.Status().Code.String()))

	return attrs
}

// matchAttributeKey 判断 key 是否在目标列表中。
func matchAttributeKey(key attribute.Key, toMatchKeys []attribute.Key) bool {
	for _, attrKey := range toMatchKeys {
		if attrKey == key {
			return true
		}
	}
	return false
}
