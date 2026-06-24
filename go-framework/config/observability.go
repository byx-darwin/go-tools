// Package config 提供可观测性统一配置。
package config

import "time"

// ObservabilityConfig 可观测性配置（OTel OTLP + 链路追踪 + 指标）。
type ObservabilityConfig struct {
	// Enabled 是否启用可观测性（Tracing + Metrics 总开关）
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Endpoint OTLP Collector 地址（如 otelcol:4317）
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// AppKey 应用密钥（环境变量注入）
	AppKey string `json:"app_key" yaml:"app_key"`

	// ServiceName 服务名称
	ServiceName string `json:"service_name" yaml:"service_name"`

	// SampleRate 采样率（0.0-1.0，默认 1.0）
	SampleRate float64 `json:"sample_rate" yaml:"sample_rate"`

	// EnableMetrics 是否启用 Metrics 导出（默认 true，当 Enabled=true 时生效）
	EnableMetrics bool `json:"enable_metrics" yaml:"enable_metrics"`

	// MetricsInterval Metrics 上报间隔（默认 15s）
	MetricsInterval time.Duration `json:"metrics_interval" yaml:"metrics_interval"`

	// EnableGRPCMetadata 启用 gRPC metadata 的 trace context 传播（transport.GRPC 协议时需开启）
	EnableGRPCMetadata bool `json:"enable_grpc_metadata" yaml:"enable_grpc_metadata"`
}
