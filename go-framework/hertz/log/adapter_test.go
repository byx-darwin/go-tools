package hertzlog_test

import (
	"context"
	"io"
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

func TestHertzAdapter_SetOutput(t *testing.T) {
	cfg := log.NewConfig()
	logger, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)

	adapter := hertzlog.NewHertzAdapter(logger)
	require.NotPanics(t, func() {
		adapter.SetOutput(io.Discard)
	})
}

func TestHertzAdapter_LoggerMethods(t *testing.T) {
	cfg := log.NewConfig()
	logger, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)

	adapter := hertzlog.NewHertzAdapter(logger)
	require.NotPanics(t, func() {
		adapter.Trace("trace", "message")
		adapter.Notice("notice", "message")
		adapter.Warn("warn", "message")
		adapter.Fatal("fatal", "message")
	})
}

func TestHertzAdapter_FormatLoggerMethods(t *testing.T) {
	cfg := log.NewConfig()
	logger, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)

	adapter := hertzlog.NewHertzAdapter(logger)
	require.NotPanics(t, func() {
		adapter.Tracef("trace %s", "message")
		adapter.Debugf("debug %s", "message")
		adapter.Infof("info %s", "message")
		adapter.Noticef("notice %s", "message")
		adapter.Warnf("warn %s", "message")
		adapter.Errorf("error %s", "message")
		adapter.Fatalf("fatal %s", "message")
	})
}

func TestHertzAdapter_CtxFormatLoggerMethods(t *testing.T) {
	cfg := log.NewConfig()
	logger, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)

	adapter := hertzlog.NewHertzAdapter(logger)
	ctx := log.WithRequestID(context.Background(), "req-123")
	require.NotPanics(t, func() {
		adapter.CtxTracef(ctx, "trace %s", "message")
		adapter.CtxDebugf(ctx, "debug %s", "message")
		adapter.CtxNoticef(ctx, "notice %s", "message")
		adapter.CtxWarnf(ctx, "warn %s", "message")
		adapter.CtxErrorf(ctx, "error %s", "message")
		adapter.CtxFatalf(ctx, "fatal %s", "message")
	})
}
