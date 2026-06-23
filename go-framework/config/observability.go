// Package config 提供可观测性统一配置。
package config

// ObservabilityConfig 可观测性配置（OTel OTLP + 链路追踪）。
type ObservabilityConfig struct {
	// Enabled 是否启用可观测性
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Endpoint OTLP Collector 地址（如 otelcol:4317）
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// AppKey 应用密钥（环境变量注入）
	AppKey string `json:"app_key" yaml:"app_key"`

	// ServiceName 服务名称
	ServiceName string `json:"service_name" yaml:"service_name"`

	// SampleRate 采样率（0.0-1.0，默认 1.0）
	SampleRate float64 `json:"sample_rate" yaml:"sample_rate"`
}
