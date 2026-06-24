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
