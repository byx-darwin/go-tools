package observability

import (
	"strings"

	"github.com/cloudwego/hertz/pkg/protocol"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.36.0"
)

// extractPeerServiceAttributesFromHTTPHeaders 从 HTTP request headers 中
// 提取上游服务信息（peer service），用于服务拓扑图。
//
// HTTP headers 中的 key 格式为 "service-name"（将 "." 替换为 "-"）：
//   - service-name → peer.service
//   - service-namespace → peer.namespace
//   - deployment-environment → peer.deployment_environment
func extractPeerServiceAttributesFromHTTPHeaders(headers *protocol.RequestHeader) []attribute.KeyValue {
	var attrs []attribute.KeyValue

	serviceName := headers.Get(semconvAttributeKeyToHTTPHeader(string(semconv.ServiceNameKey)))
	serviceNamespace := headers.Get(semconvAttributeKeyToHTTPHeader(string(semconv.ServiceNamespaceKey)))
	deploymentEnv := headers.Get(semconvAttributeKeyToHTTPHeader(string(semconv.DeploymentEnvironmentNameKey)))

	if serviceName != "" {
		attrs = append(attrs, semconv.PeerServiceKey.String(serviceName))
	}
	if serviceNamespace != "" {
		attrs = append(attrs, PeerServiceNamespaceKey.String(serviceNamespace))
	}
	if deploymentEnv != "" {
		attrs = append(attrs, PeerDeploymentEnvironmentKey.String(deploymentEnv))
	}

	return attrs
}

// semconvAttributeKeyToHTTPHeader 将 semconv key 中的 "." 替换为 "-"，
// 适配 HTTP header 命名规范（如 service.name → service-name）。
func semconvAttributeKeyToHTTPHeader(key string) string {
	return strings.ReplaceAll(key, ".", "-")
}

// injectPeerServiceToHTTPHeaders 将当前服务的 resource attributes 注入到
// outgoing HTTP headers 中，使下游服务能够通过 extractPeerServiceAttributesFromHTTPHeaders
// 提取对端服务信息，用于服务拓扑图。
func injectPeerServiceToHTTPHeaders(attrs []attribute.KeyValue, header *protocol.RequestHeader) {
	serviceName, serviceNamespace, deploymentEnv := getServiceFromResourceAttributes(attrs)

	if serviceName != "" {
		header.Set(semconvAttributeKeyToHTTPHeader(string(semconv.ServiceNameKey)), serviceName)
	}
	if serviceNamespace != "" {
		header.Set(semconvAttributeKeyToHTTPHeader(string(semconv.ServiceNamespaceKey)), serviceNamespace)
	}
	if deploymentEnv != "" {
		header.Set(semconvAttributeKeyToHTTPHeader(string(semconv.DeploymentEnvironmentNameKey)), deploymentEnv)
	}
}

// getServiceFromResourceAttributes 从 resource attributes 中提取服务标识。
func getServiceFromResourceAttributes(attrs []attribute.KeyValue) (serviceName, serviceNamespace, deploymentEnv string) {
	for _, attr := range attrs {
		switch attr.Key {
		case semconv.ServiceNameKey:
			serviceName = attr.Value.AsString()
		case semconv.ServiceNamespaceKey:
			serviceNamespace = attr.Value.AsString()
		case semconv.DeploymentEnvironmentNameKey:
			deploymentEnv = attr.Value.AsString()
		}
	}
	return
}
