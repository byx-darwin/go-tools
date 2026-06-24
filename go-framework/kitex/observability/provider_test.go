package observability

import (
	"context"
	"testing"

	"github.com/byx-darwin/go-tools/go-framework/config"
	"github.com/stretchr/testify/assert"
)

func TestNewProvider(t *testing.T) {
	ctx := context.Background()
	p, err := NewProvider(ctx, config.ObservabilityConfig{
		Enabled: true,
	})
	assert.NoError(t, err)
	assert.True(t, p.Enabled())
}

func TestProvider_Disabled(t *testing.T) {
	p, err := NewProvider(context.Background(), config.ObservabilityConfig{
		Enabled: false,
	})
	assert.NoError(t, err)
	assert.False(t, p.Enabled())

	// Middleware should be a pass-through when disabled
	mw := p.Middleware()
	assert.NotNil(t, mw)

	called := false
	next := func(ctx context.Context, req, resp interface{}) error {
		called = true
		return nil
	}
	err = mw(next)(nil, nil, nil)
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestProvider_Shutdown(t *testing.T) {
	p, _ := NewProvider(context.Background(), config.ObservabilityConfig{})
	assert.NoError(t, p.Shutdown())
}
