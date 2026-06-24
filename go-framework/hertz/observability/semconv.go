// Package observability 提供 Hertz HTTP 服务的 OpenTelemetry 集成。
package observability

import "go.opentelemetry.io/otel/attribute"

// HTTP metric 名称。
const (
	// ServerRequestCount 服务端 HTTP 请求计数（Counter）。
	ServerRequestCount = "http.server.request_count"

	// ServerLatency 服务端 HTTP 请求耗时（Histogram，单位 ms）。
	ServerLatency = "http.server.duration"

	// ClientRequestCount 客户端 HTTP 请求计数（Counter）。
	ClientRequestCount = "http.client.request_count"

	// ClientLatency 客户端 HTTP 请求耗时（Histogram，单位 ms）。
	ClientLatency = "http.client.duration"
)

// HTTP 特定 attribute keys。
const (
	// RequestProtocolKey 请求协议标识。
	RequestProtocolKey = attribute.Key("request.protocol")

	// PeerServiceNamespaceKey peer.service.namespace
	PeerServiceNamespaceKey = attribute.Key("peer.service.namespace")

	// PeerDeploymentEnvironmentKey peer.deployment.environment
	PeerDeploymentEnvironmentKey = attribute.Key("peer.deployment.environment")

	// StatusKey span 状态码（OK / Error）。
	StatusKey = attribute.Key("status.code")
)
