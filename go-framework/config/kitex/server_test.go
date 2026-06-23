package kitex

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestServerConfig_Defaults(t *testing.T) {
	c := &ServerConfig{}
	assert.Nil(t, c.RPC)
	assert.Nil(t, c.Timeout)
}

func TestServerConfig_Full(t *testing.T) {
	c := &ServerConfig{
		RPC: &RPCOption{
			Port:    "8888",
			Network: "tcp",
		},
		Limit: &LimitOption{
			Enable:         true,
			MaxConnections: 10000,
			MaxQPS:         5000,
		},
		Timeout: &ServerTimeout{
			ReadWriteTimeout: 5 * time.Second,
			ExitWaitTimeout:  5 * time.Second,
		},
	}

	assert.Equal(t, "8888", c.RPC.Port)
	assert.Equal(t, "tcp", c.RPC.Network)
	assert.True(t, c.Limit.Enable)
	assert.Equal(t, 10000, c.Limit.MaxConnections)
	assert.Equal(t, 5000, c.Limit.MaxQPS)
	assert.Equal(t, 5*time.Second, c.Timeout.ReadWriteTimeout)
	assert.Equal(t, 5*time.Second, c.Timeout.ExitWaitTimeout)
}

func TestServerConfig_DurationFields(t *testing.T) {
	// Per D2: time fields should be time.Duration
	c := &ServerTimeout{
		ReadWriteTimeout: 3 * time.Second,
		ExitWaitTimeout:  10 * time.Second,
	}
	assert.Equal(t, 3*time.Second, c.ReadWriteTimeout)
	assert.Equal(t, 10*time.Second, c.ExitWaitTimeout)
}
