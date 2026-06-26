package kitexlog_test

import (
	"context"
	"testing"

	"github.com/byx-darwin/go-tools/go-common/log"
	kitexlog "github.com/byx-darwin/go-tools/go-framework/kitex/log"
	"github.com/stretchr/testify/require"
)

func TestKitexAdapter_Info(t *testing.T) {
	cfg := log.NewConfig()
	logger, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)

	adapter := kitexlog.NewKitexAdapter(logger)
	require.NotPanics(t, func() {
		adapter.Info(context.Background(), "test message")
	})
}

func TestKitexAdapter_Debug(t *testing.T) {
	cfg := log.NewConfig()
	logger, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)

	adapter := kitexlog.NewKitexAdapter(logger)
	require.NotPanics(t, func() {
		adapter.Debug(context.Background(), "debug message")
	})
}

func TestKitexAdapter_Error(t *testing.T) {
	cfg := log.NewConfig()
	logger, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)

	adapter := kitexlog.NewKitexAdapter(logger)
	require.NotPanics(t, func() {
		adapter.Error(context.Background(), "error message")
	})
}

func TestKitexAdapter_CtxInfof(t *testing.T) {
	cfg := log.NewConfig()
	logger, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)

	adapter := kitexlog.NewKitexAdapter(logger)
	ctx := log.WithRequestID(context.Background(), "req-123")
	require.NotPanics(t, func() {
		adapter.CtxInfof(ctx, "test %s", "message")
	})
}

func TestKitexAdapter_SetLevel(t *testing.T) {
	cfg := log.NewConfig()
	logger, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)

	adapter := kitexlog.NewKitexAdapter(logger)
	require.NotPanics(t, func() {
		adapter.SetLevel(0)
	})
}
