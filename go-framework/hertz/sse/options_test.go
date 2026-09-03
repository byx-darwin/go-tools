package sse

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	c := defaultConfig()
	assert.Equal(t, 15*time.Second, c.heartbeatInterval)
	assert.Nil(t, c.onRecover)
}

func TestWithHeartbeatInterval(t *testing.T) {
	c := defaultConfig()
	WithHeartbeatInterval(30 * time.Second)(&c)
	assert.Equal(t, 30*time.Second, c.heartbeatInterval)

	// <=0 disables heartbeat.
	WithHeartbeatInterval(0)(&c)
	assert.Equal(t, time.Duration(0), c.heartbeatInterval)

	WithHeartbeatInterval(-1 * time.Second)(&c)
	assert.Equal(t, time.Duration(-1*time.Second), c.heartbeatInterval)
}

func TestWithRecoverHandler(t *testing.T) {
	c := defaultConfig()
	called := false
	WithRecoverHandler(func(rec any) { called = true })(&c)
	assert.NotNil(t, c.onRecover)
	c.onRecover("boom")
	assert.True(t, called)

	// nil handler is a no-op, doesn't clear existing handler.
	WithRecoverHandler(nil)(&c)
	assert.NotNil(t, c.onRecover)
}
