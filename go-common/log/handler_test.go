package log_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/byx-darwin/go-tools/go-common/log"
	"github.com/stretchr/testify/require"
)

func TestCategoryHandler(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	handler := log.NewCategoryHandler(inner, "access")

	logger := slog.New(handler)
	logger.InfoContext(context.Background(), "test")

	require.Contains(t, buf.String(), `"category":"access"`)
	require.Contains(t, buf.String(), `"test"`)
}

func TestReleaseHandler(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	release := log.ReleaseInfo{
		ServiceName: "test-service",
		Version:     "v1.0.0",
		Environment: "production",
	}
	handler := log.NewReleaseHandler(inner, release)

	logger := slog.New(handler)
	logger.InfoContext(context.Background(), "test")

	require.Contains(t, buf.String(), `"service.name":"test-service"`)
	require.Contains(t, buf.String(), `"service.version":"v1.0.0"`)
	require.Contains(t, buf.String(), `"environment":"production"`)
}

func TestReleaseHandler_WithExtra(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	release := log.ReleaseInfo{
		ServiceName: "test-service",
	}.WithExtra("region", "us-west-2")
	handler := log.NewReleaseHandler(inner, release)

	logger := slog.New(handler)
	logger.InfoContext(context.Background(), "test")

	require.Contains(t, buf.String(), `"region":"us-west-2"`)
}

func TestContextHandler(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	handler := log.NewContextHandler(inner)

	ctx := log.WithRequestID(context.Background(), "req-123")
	logger := slog.New(handler)
	logger.InfoContext(ctx, "test")

	require.Contains(t, buf.String(), `"request_id":"req-123"`)
}

func TestContextHandler_NoRequestID(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	handler := log.NewContextHandler(inner)

	logger := slog.New(handler)
	logger.InfoContext(context.Background(), "test")

	require.NotContains(t, buf.String(), "request_id")
}

func TestMaskHandler(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	masker := log.NewMasker(log.MaskConfig{
		Enabled:      true,
		MaskedFields: []string{"password"},
		Mode:         "full",
	})
	handler := log.NewMaskHandler(inner, masker)

	logger := slog.New(handler)
	logger.InfoContext(context.Background(), "test", "password", "secret123")

	require.Contains(t, buf.String(), `"password":"***"`)
	require.NotContains(t, buf.String(), "secret123")
}

func TestMultiHandler(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	h1 := slog.NewJSONHandler(&buf1, &slog.HandlerOptions{})
	h2 := slog.NewJSONHandler(&buf2, &slog.HandlerOptions{})
	handler := log.NewMultiHandler(h1, h2)

	logger := slog.New(handler)
	logger.InfoContext(context.Background(), "test")

	require.Contains(t, buf1.String(), "test")
	require.Contains(t, buf2.String(), "test")
}

func TestMultiHandler_Enabled(t *testing.T) {
	h1 := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})
	h2 := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelDebug})
	handler := log.NewMultiHandler(h1, h2)

	// 至少有一个 handler 启用就应该返回 true
	require.True(t, handler.Enabled(context.Background(), slog.LevelInfo))
}
