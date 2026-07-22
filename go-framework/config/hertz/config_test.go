package hertz

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestServerConfig_Defaults(t *testing.T) {
	c := &ServerConfig{}
	assert.Nil(t, c.HTTP)
	assert.Nil(t, c.Auth)
}

func TestServerConfig_Full(t *testing.T) {
	c := &ServerConfig{
		HTTP: &HTTPOption{
			Network:      "tcp",
			Port:         "8080",
			Mode:         0,
			ExitWaitTime: 5 * time.Second,
			IdleTimeout:  30 * time.Second,
			IsTransport:  true,
			IsCors:       true,
			IsRecovery:   true,
		},
		Auth: &HTTPAuth{
			Enable: true,
			AK:     "test-ak",
			SK:     "test-sk",
		},
	}

	assert.Equal(t, "tcp", c.HTTP.Network)
	assert.Equal(t, "8080", c.HTTP.Port)
	assert.Equal(t, 5*time.Second, c.HTTP.ExitWaitTime)
	assert.Equal(t, 30*time.Second, c.HTTP.IdleTimeout)
	assert.True(t, c.HTTP.IsCors)
	assert.True(t, c.HTTP.IsRecovery)
	assert.True(t, c.Auth.Enable)
	assert.Equal(t, "test-ak", c.Auth.AK)
	assert.Equal(t, "test-sk", c.Auth.SK)
}

func TestHTTPOption_Mode(t *testing.T) {
	// Mode 0 = internal, Mode 1 = external
	internal := HTTPOption{Mode: 0}
	external := HTTPOption{Mode: 1}
	assert.Equal(t, 0, internal.Mode)
	assert.Equal(t, 1, external.Mode)
}
