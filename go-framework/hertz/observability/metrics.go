package observability

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.36.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// httpMetricsAttributes HTTP metrics 应携带的属性 key。
var httpMetricsAttributes = []attribute.Key{
	attribute.Key("http.host"),
	attribute.Key("http.route"),
	attribute.Key("http.request.method"),
	attribute.Key("http.response.status_code"),
}

// peerMetricsAttributes 对端服务的属性 key。
var peerMetricsAttributes = []attribute.Key{
	semconv.PeerServiceKey,
	PeerServiceNamespaceKey,
	PeerDeploymentEnvironmentKey,
	RequestProtocolKey,
}

// metricResourceAttributes resource 级别的 metrics 属性 key。
var metricResourceAttributes = []attribute.Key{
	semconv.ServiceNameKey,
	semconv.ServiceNamespaceKey,
	semconv.DeploymentEnvironmentNameKey,
	semconv.ServiceInstanceIDKey,
	semconv.ServiceVersionKey,
	semconv.TelemetrySDKLanguageKey,
	semconv.TelemetrySDKVersionKey,
	semconv.ProcessPIDKey,
	semconv.HostNameKey,
	semconv.HostIDKey,
}

// extractMetricsAttributesFromSpan 从 span 中提取用于 HTTP metrics 记录的属性。
func extractMetricsAttributesFromSpan(span oteltrace.Span) []attribute.KeyValue {
	var attrs []attribute.KeyValue
	readOnlySpan, ok := span.(trace.ReadOnlySpan)
	if !ok {
		return attrs
	}

	for _, attr := range readOnlySpan.Attributes() {
		if matchAttributeKey(attr.Key, httpMetricsAttributes) {
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

func matchAttributeKey(key attribute.Key, toMatchKeys []attribute.Key) bool {
	for _, attrKey := range toMatchKeys {
		if attrKey == key {
			return true
		}
	}
	return false
}
