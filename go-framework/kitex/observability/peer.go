package observability

import (
	"context"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// injectPeerServiceToMetaInfo 将当前服务的 resource attributes 注入到 metainfo 中，
// 使下游服务能够识别调用方（peer service），用于服务拓扑图。
//
// 注入的 key：
//   - service.name → peer.service（下游提取时转换）
//   - service.namespace → peer.namespace
//   - deployment.environment → peer.deployment_environment
func injectPeerServiceToMetaInfo(ctx context.Context, attrs []attribute.KeyValue) {
	md := metainfo.GetAllValues(ctx)
	if md == nil {
		md = make(map[string]string)
	}

	serviceName, serviceNamespace, deploymentEnv := getServiceFromResourceAttributes(attrs)

	if serviceName != "" {
		md[string(semconv.ServiceNameKey)] = serviceName
	}
	if serviceNamespace != "" {
		md[string(semconv.ServiceNamespaceKey)] = serviceNamespace
	}
	if deploymentEnv != "" {
		md[string(semconv.DeploymentEnvironmentKey)] = deploymentEnv
	}

	// 将构建好的 metadata 写回 context（通过 backward transfer 传递到下游）
	_ = metainfo.SendBackwardValuesFromMap(ctx, md)
}

// extractPeerServiceAttributesFromMetaInfo 从 metainfo map 中提取上游服务的属性。
//
// 提取后映射为 OTel span attributes：
//   - service.name → peer.service
//   - service.namespace → peer.namespace
//   - deployment.environment → peer.deployment_environment
func extractPeerServiceAttributesFromMetaInfo(md map[string]string) []attribute.KeyValue {
	var attrs []attribute.KeyValue

	for k, v := range md {
		switch attribute.Key(k) {
		case semconv.ServiceNameKey:
			attrs = append(attrs, semconv.PeerServiceKey.String(v))
		case semconv.ServiceNamespaceKey:
			attrs = append(attrs, PeerServiceNamespaceKey.String(v))
		case semconv.DeploymentEnvironmentKey:
			attrs = append(attrs, PeerDeploymentEnvironmentKey.String(v))
		}
	}

	return attrs
}

// getServiceFromResourceAttributes 从 resource attributes 中提取服务标识信息。
func getServiceFromResourceAttributes(attrs []attribute.KeyValue) (serviceName, serviceNamespace, deploymentEnv string) {
	for _, attr := range attrs {
		switch attr.Key {
		case semconv.ServiceNameKey:
			serviceName = attr.Value.AsString()
		case semconv.ServiceNamespaceKey:
			serviceNamespace = attr.Value.AsString()
		case semconv.DeploymentEnvironmentKey:
			deploymentEnv = attr.Value.AsString()
		}
	}
	return
}
