package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccessLog_Kitex(t *testing.T) {
	mw := AccessLog()
	assert.NotNil(t, mw)

	// Test middleware chain
	endpoint := func(ctx context.Context, req, resp any) error {
		return nil
	}

	wrapped := mw(endpoint)
	err := wrapped(context.Background(), nil, nil)
	assert.NoError(t, err)
}

func TestAccessLog_Kitex_Error(t *testing.T) {
	mw := AccessLog()
	endpoint := func(ctx context.Context, req, resp any) error {
		return errors.New("rpc error")
	}

	wrapped := mw(endpoint)
	err := wrapped(context.Background(), nil, nil)
	assert.Error(t, err)
}

func TestAccessLog_Kitex_TypeCompatibility(t *testing.T) {
	// Verify Middleware type is compatible with kitex endpoint.Middleware
	var mw Middleware = AccessLog()
	assert.NotNil(t, mw)

	// Verify Endpoint type
	var ep Endpoint = func(ctx context.Context, req, resp any) error {
		return nil
	}
	assert.NotNil(t, ep)
}
