// Package observability 提供 Kitex RPC 服务的 OpenTelemetry 集成。
package observability

import (
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// RPC metric 名称。
const (
	// ServerDuration 服务端 RPC 耗时（Histogram，单位 ms）。
	ServerDuration = "rpc.server.duration"

	// ClientDuration 客户端 RPC 耗时（Histogram，单位 ms）。
	ClientDuration = "rpc.client.duration"
)

// RPCSystemKitex RPC 系统标识（值为 "kitex"）。
var RPCSystemKitex = semconv.RPCSystemKey.String("kitex")

// 自定义 attribute keys（otel semconv 未覆盖的 Kitex 特定属性）。
var (
	// RequestProtocolKey RPC 请求协议（如 TTHeader、Framed）。
	RequestProtocolKey = attribute.Key("rpc.kitex.protocol")

	// RPCSystemKitexRecvSize 服务端收包字节数。
	RPCSystemKitexRecvSize = attribute.Key("rpc.kitex.recv_size")

	// RPCSystemKitexSendSize 服务端发包字节数。
	RPCSystemKitexSendSize = attribute.Key("rpc.kitex.send_size")

	// SourceOperationKey 调用方操作名（可能造成高基数，可选开启）。
	SourceOperationKey = attribute.Key("rpc.kitex.source_operation")

	// PeerServiceNamespaceKey 对端服务命名空间。
	PeerServiceNamespaceKey = attribute.Key("peer.namespace")

	// PeerDeploymentEnvironmentKey 对端部署环境。
	PeerDeploymentEnvironmentKey = attribute.Key("peer.deployment_environment")

	// StatusKey span 状态码（OK / Error）。
	StatusKey = attribute.Key("status_code")
)
