package kitex

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClientConfig_Defaults(t *testing.T) {
	c := &ClientConfig{}
	assert.Nil(t, c.RPC)
	assert.Nil(t, c.ClientOption)
}

func TestClientConfig_Full(t *testing.T) {
	c := &ClientConfig{
		RPC: &RPCServerOption{
			Name:     "target-service",
			Intranet: "10.0.0.1:8888",
		},
		ClientOption: &ClientOption{
			Resolver: ResolverOption{
				Enable:  true,
				Space:   "prod",
				Name:    "target-service",
				Version: "v1",
				Env:     "prod",
			},
			MuxConnNum: 4,
			Timeout: ClientTimeout{
				RPCTimeout:     3 * time.Second,
				ConnectTimeOut: 50 * time.Millisecond,
			},
			LoadBalancer: LoadBalancer{Enable: true},
			CBSuite:      CBSuite{Enable: true},
			ConnPool: ConnPool{
				MinIdlePerAddress: 1,
				MaxIdlePerAddress: 10,
				MaxIdleGlobal:     1000,
				MaxIdleTimeout:    30 * time.Second,
			},
		},
	}

	assert.Equal(t, "target-service", c.RPC.Name)
	assert.Equal(t, 4, c.ClientOption.MuxConnNum)
	assert.Equal(t, 3*time.Second, c.ClientOption.Timeout.RPCTimeout)
	assert.Equal(t, 50*time.Millisecond, c.ClientOption.Timeout.ConnectTimeOut)
	assert.True(t, c.ClientOption.LoadBalancer.Enable)
	assert.True(t, c.ClientOption.CBSuite.Enable)
	assert.Equal(t, 1, c.ClientOption.ConnPool.MinIdlePerAddress)
	assert.Equal(t, 10, c.ClientOption.ConnPool.MaxIdlePerAddress)
	assert.Equal(t, 1000, c.ClientOption.ConnPool.MaxIdleGlobal)
	assert.Equal(t, 30*time.Second, c.ClientOption.ConnPool.MaxIdleTimeout)
}

func TestClientConfig_DurationFields(t *testing.T) {
	ct := ClientTimeout{
		RPCTimeout:     5 * time.Second,
		ConnectTimeOut: 100 * time.Millisecond,
	}
	assert.Equal(t, 5*time.Second, ct.RPCTimeout)
	assert.Equal(t, 100*time.Millisecond, ct.ConnectTimeOut)
}

func TestFailureRetry_BackOffDefaults(t *testing.T) {
	fr := FailureRetry{}
	assert.Equal(t, "", fr.BackOff.Type)
	assert.Equal(t, 0, fr.BackOff.FixedMS)
	assert.Equal(t, 0, fr.BackOff.MinMS)
	assert.Equal(t, 0, fr.BackOff.MaxMS)
}

func TestFailureRetry_BackOffFixed(t *testing.T) {
	fr := FailureRetry{
		Enable:        true,
		MaxRetryTimes: 2,
		BackOff:       BackOff{Type: "fixed", FixedMS: 50},
	}
	assert.Equal(t, "fixed", fr.BackOff.Type)
	assert.Equal(t, 50, fr.BackOff.FixedMS)
}

func TestFailureRetry_BackOffRandom(t *testing.T) {
	fr := FailureRetry{
		Enable:        true,
		MaxRetryTimes: 2,
		BackOff:       BackOff{Type: "random", MinMS: 10, MaxMS: 100},
	}
	assert.Equal(t, "random", fr.BackOff.Type)
	assert.Equal(t, 10, fr.BackOff.MinMS)
	assert.Equal(t, 100, fr.BackOff.MaxMS)
}
