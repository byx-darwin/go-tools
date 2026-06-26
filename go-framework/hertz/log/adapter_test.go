package hertzlog_test

import (
	"context"
	"testing"

	"github.com/byx-darwin/go-tools/go-common/log"
	hertzlog "github.com/byx-darwin/go-tools/go-framework/hertz/log"
	"github.com/stretchr/testify/require"
)

func TestHertzAdapter_Info(t *testing.T) {
	cfg := log.NewConfig()
	logger, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)

	adapter := hertzlog.NewHertzAdapter(logger)
	require.NotPanics(t, func() {
		adapter.Info(context.Background(), "test message")
	})
}

func TestHertzAdapter_Debug(t *testing.T) {
	cfg := log.NewConfig()
	logger, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)

	adapter := hertzlog.NewHertzAdapter(logger)
	require.NotPanics(t, func() {
		adapter.Debug(context.Background(), "debug message")
	})
}

func TestHertzAdapter_Error(t *testing.T) {
	cfg := log.NewConfig()
	logger, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)

	adapter := hertzlog.NewHertzAdapter(logger)
	require.NotPanics(t, func() {
		adapter.Error(context.Background(), "error message")
	})
}

func TestHertzAdapter_CtxInfof(t *testing.T) {
	cfg := log.NewConfig()
	logger, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)

	adapter := hertzlog.NewHertzAdapter(logger)
	ctx := log.WithRequestID(context.Background(), "req-123")
	require.NotPanics(t, func() {
		adapter.CtxInfof(ctx, "test %s", "message")
	})
}

func TestHertzAdapter_SetLevel(t *testing.T) {
	cfg := log.NewConfig()
	logger, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)

	adapter := hertzlog.NewHertzAdapter(logger)
	require.NotPanics(t, func() {
		adapter.SetLevel(0)
	})
}
