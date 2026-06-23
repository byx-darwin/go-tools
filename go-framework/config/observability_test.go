package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObservabilityConfig_Defaults(t *testing.T) {
	c := ObservabilityConfig{}
	assert.False(t, c.Enabled)
	assert.Empty(t, c.Endpoint)
	assert.Equal(t, 0.0, c.SampleRate)
}

func TestObservabilityConfig_Full(t *testing.T) {
	c := ObservabilityConfig{
		Enabled:     true,
		Endpoint:    "otelcol:4317",
		AppKey:      "key-123",
		ServiceName: "my-service",
		SampleRate:  0.5,
	}
	assert.True(t, c.Enabled)
	assert.Equal(t, "otelcol:4317", c.Endpoint)
	assert.Equal(t, "key-123", c.AppKey)
	assert.Equal(t, "my-service", c.ServiceName)
	assert.Equal(t, 0.5, c.SampleRate)
}
