package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestObservabilityConfig_Defaults(t *testing.T) {
	c := ObservabilityConfig{}
	assert.False(t, c.Enabled)
	assert.Empty(t, c.Endpoint)
	assert.Equal(t, 0.0, c.SampleRate)
	assert.False(t, c.EnableMetrics)
	assert.Equal(t, time.Duration(0), c.MetricsInterval)
}

func TestObservabilityConfig_Full(t *testing.T) {
	c := ObservabilityConfig{
		Enabled:            true,
		Endpoint:           "otelcol:4317",
		AppKey:             "key-123",
		ServiceName:        "my-service",
		SampleRate:         0.5,
		EnableMetrics:      true,
		MetricsInterval:    15 * time.Second,
		EnableGRPCMetadata: true,
	}
	assert.True(t, c.Enabled)
	assert.Equal(t, "otelcol:4317", c.Endpoint)
	assert.Equal(t, "key-123", c.AppKey)
	assert.Equal(t, "my-service", c.ServiceName)
	assert.Equal(t, 0.5, c.SampleRate)
	assert.True(t, c.EnableMetrics)
	assert.Equal(t, 15*time.Second, c.MetricsInterval)
	assert.True(t, c.EnableGRPCMetadata)
}

func TestObservabilityConfig_MetricsDefaults(t *testing.T) {
	c := ObservabilityConfig{Enabled: true}
	// EnableMetrics 默认 false，但 Enabled=true 时按 true 处理
	assert.False(t, c.EnableMetrics)
	// MetricsInterval 默认 0，由 Provider 补全为 15s
	assert.Equal(t, time.Duration(0), c.MetricsInterval)
}
