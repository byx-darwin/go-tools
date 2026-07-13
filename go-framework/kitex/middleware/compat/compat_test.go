package compat

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccessLog_ReturnsMiddleware(t *testing.T) {
	mw := AccessLog()
	require.NotNil(t, mw)
}

func TestAccessLog_WrapsEndpoint(t *testing.T) {
	mw := AccessLog()
	called := false
	next := func(ctx context.Context, req, resp any) error {
		called = true
		return nil
	}
	wrapped := mw(next)
	require.NotNil(t, wrapped)

	err := wrapped(context.Background(), nil, nil)
	assert.NoError(t, err)
	assert.True(t, called)
}
