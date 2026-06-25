package log_test

import (
	"context"
	"testing"

	"github.com/byx-darwin/go-tools/go-common/log"
	"github.com/stretchr/testify/require"
)

func TestGlobal_InitAndL(t *testing.T) {
	cfg := log.Config{
		Level:  "info",
		Format: "json",
		Mode:   "console",
	}
	release := log.ReleaseInfo{
		ServiceName: "test-service",
		Version:     "v1.0.0",
	}

	err := log.Init(cfg, release)
	require.NoError(t, err)
	defer func() { _ = log.Close() }()

	logger := log.L()
	require.NotNil(t, logger)

	logger.InfoContext(context.Background(), "test message")
}

func TestGlobal_SetDefault(t *testing.T) {
	cfg := log.Config{
		Level:  "info",
		Format: "json",
		Mode:   "console",
	}
	logger, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)

	log.SetDefault(logger)
	defer func() { _ = log.Close() }()

	l := log.L()
	require.NotNil(t, l)
}

func TestGlobal_L_BeforeInit(t *testing.T) {
	// 确保 L() 在 Init 之前调用不会 panic
	_ = log.Close()
	logger := log.L()
	require.NotNil(t, logger)
}

func TestGlobal_Close(t *testing.T) {
	cfg := log.Config{
		Level:  "info",
		Format: "json",
		Mode:   "console",
	}
	err := log.Init(cfg, log.ReleaseInfo{})
	require.NoError(t, err)

	err = log.Close()
	require.NoError(t, err)
}

func TestGlobal_WithCategory(t *testing.T) {
	cfg := log.Config{
		Level:  "info",
		Format: "json",
		Mode:   "console",
	}
	err := log.Init(cfg, log.ReleaseInfo{ServiceName: "test"})
	require.NoError(t, err)
	defer func() { _ = log.Close() }()

	accessLog := log.L().WithCategory(log.CategoryAccess)
	require.NotNil(t, accessLog)

	accessLog.InfoContext(context.Background(), "access log test")
}
