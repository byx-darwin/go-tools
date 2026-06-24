package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/byx-darwin/go-tools/go-common/log"
	"github.com/stretchr/testify/assert"
)

func TestAccessLog_Kitex(t *testing.T) {
	logger := log.NewFromConfig(log.Config{Level: "error"})
	defer logger.Close()

	mw := AccessLog(logger)
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
	logger := log.NewFromConfig(log.Config{Level: "error"})
	defer logger.Close()

	mw := AccessLog(logger)
	endpoint := func(ctx context.Context, req, resp any) error {
		return errors.New("rpc error")
	}

	wrapped := mw(endpoint)
	err := wrapped(context.Background(), nil, nil)
	assert.Error(t, err)
}

func TestAccessLog_Kitex_TypeCompatibility(t *testing.T) {
	// Verify Middleware type is compatible with kitex endpoint.Middleware
	var mw Middleware = AccessLog(log.NewFromConfig(log.Config{}))
	assert.NotNil(t, mw)

	// Verify Endpoint type
	var ep Endpoint = func(ctx context.Context, req, resp any) error {
		return nil
	}
	assert.NotNil(t, ep)
}
