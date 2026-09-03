package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/byx-darwin/go-tools/go-framework/kitex/rpcerror"
)

func TestRecovery_CatchesPanic(t *testing.T) {
	mw := Recovery()
	panicking := func(ctx context.Context, req, resp any) error {
		panic("boom")
	}

	wrapped := mw(panicking)

	var err error
	assert.NotPanics(t, func() {
		err = wrapped(context.Background(), nil, nil)
	})
	require.Error(t, err)

	adapter, ok := err.(*rpcerror.OopsStatusAdapter)
	require.True(t, ok)
	assert.Contains(t, adapter.Error(), "boom")
}

func TestRecovery_PassesThroughSuccess(t *testing.T) {
	mw := Recovery()
	ok := func(ctx context.Context, req, resp any) error { return nil }

	wrapped := mw(ok)
	err := wrapped(context.Background(), nil, nil)
	require.NoError(t, err)
}

func TestRecovery_PassesThroughError(t *testing.T) {
	mw := Recovery()
	failing := func(ctx context.Context, req, resp any) error { return assert.AnError }

	wrapped := mw(failing)
	err := wrapped(context.Background(), nil, nil)
	require.Error(t, err)
	assert.Equal(t, assert.AnError, err)
}
