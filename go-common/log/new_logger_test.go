package log_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/byx-darwin/go-tools/go-common/log"
	"github.com/samber/oops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger_ConsoleMode(t *testing.T) {
	cfg := log.Config{
		Level:  "info",
		Format: "json",
		Mode:   "console",
	}
	l, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)
	require.NotNil(t, l)
	l.InfoContext(context.Background(), "console test")
}

func TestNewLogger_FileMode_JSON(t *testing.T) {
	dir := t.TempDir()
	cfg := log.Config{
		Level:  "info",
		Format: "json",
		Mode:   "file",
		File: log.FileConfig{
			Dir:      dir,
			Filename: "test.log",
		},
	}
	l, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)
	defer func() { _ = l.Close() }()

	l.InfoContext(context.Background(), "file json test", "key", "val")

	data, err := os.ReadFile(filepath.Join(dir, "test.log"))
	require.NoError(t, err)

	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &entry))
	assert.Equal(t, "file json test", entry["msg"])
	assert.Equal(t, "val", entry["key"])
}

func TestNewLogger_FileMode_Text(t *testing.T) {
	dir := t.TempDir()
	cfg := log.Config{
		Level:  "debug",
		Format: "text",
		Mode:   "file",
		File: log.FileConfig{
			Dir:      dir,
			Filename: "test.log",
		},
	}
	l, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)
	defer func() { _ = l.Close() }()

	l.DebugContext(context.Background(), "file text test")

	data, err := os.ReadFile(filepath.Join(dir, "test.log"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "file text test")
}

func TestNewLogger_BothMode(t *testing.T) {
	dir := t.TempDir()
	cfg := log.Config{
		Level:  "info",
		Format: "json",
		Mode:   "both",
		File: log.FileConfig{
			Dir:      dir,
			Filename: "test.log",
		},
	}
	l, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)
	defer func() { _ = l.Close() }()

	l.InfoContext(context.Background(), "both mode test")

	data, err := os.ReadFile(filepath.Join(dir, "test.log"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "both mode test")
}

func TestNewLogger_FileMode_EmptyPath_FallbackStdout(t *testing.T) {
	cfg := log.Config{
		Level:  "info",
		Format: "json",
		Mode:   "file",
		File:   log.FileConfig{}, // 空路径
	}
	l, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)
	require.NotNil(t, l)
	// 不会 panic，回退到 stdout
	l.InfoContext(context.Background(), "fallback test")
}

func TestNewLogger_WithReleaseInfo(t *testing.T) {
	dir := t.TempDir()
	cfg := log.Config{
		Level:  "info",
		Format: "json",
		Mode:   "file",
		File: log.FileConfig{
			Dir:      dir,
			Filename: "test.log",
		},
	}
	release := log.ReleaseInfo{
		ServiceName: "my-service",
		Version:     "v2.0.0",
		Environment: "production",
	}
	l, err := log.NewLogger(cfg, release)
	require.NoError(t, err)
	defer func() { _ = l.Close() }()

	l.InfoContext(context.Background(), "release test")

	data, err := os.ReadFile(filepath.Join(dir, "test.log"))
	require.NoError(t, err)

	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &entry))
	assert.Equal(t, "my-service", entry["service.name"])
	assert.Equal(t, "v2.0.0", entry["service.version"])
	assert.Equal(t, "production", entry["environment"])
}

func TestNewLogger_WithContextRequestID(t *testing.T) {
	dir := t.TempDir()
	cfg := log.Config{
		Level:  "info",
		Format: "json",
		Mode:   "file",
		File: log.FileConfig{
			Dir:      dir,
			Filename: "test.log",
		},
	}
	l, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)
	defer func() { _ = l.Close() }()

	ctx := log.WithRequestID(context.Background(), "req-456")
	l.InfoContext(ctx, "request id test")

	data, err := os.ReadFile(filepath.Join(dir, "test.log"))
	require.NoError(t, err)

	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &entry))
	assert.Equal(t, "req-456", entry["request_id"])
}

func TestNewLogger_WithMasking(t *testing.T) {
	dir := t.TempDir()
	cfg := log.Config{
		Level:  "info",
		Format: "json",
		Mode:   "file",
		File: log.FileConfig{
			Dir:      dir,
			Filename: "test.log",
		},
		Masking: log.MaskConfig{
			Enabled:      true,
			MaskedFields: []string{"password"},
			Mode:         "full",
		},
	}
	l, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)
	defer func() { _ = l.Close() }()

	l.InfoContext(context.Background(), "mask test", "password", "secret123")

	data, err := os.ReadFile(filepath.Join(dir, "test.log"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"password":"***"`)
	assert.NotContains(t, string(data), "secret123")
}

func TestLogger_ErrorContext_WithOopsError(t *testing.T) {
	dir := t.TempDir()
	cfg := log.Config{
		Level:  "debug",
		Format: "json",
		Mode:   "file",
		File: log.FileConfig{
			Dir:      dir,
			Filename: "test.log",
		},
	}
	l, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)
	defer func() { _ = l.Close() }()

	oopsErr := oops.Code("DB_ERROR").In("database").Hint("check connection").New("connection failed")

	ctx := context.Background()
	l.ErrorContext(ctx, "oops error test", oopsErr, "extra_key", "extra_val")

	data, err := os.ReadFile(filepath.Join(dir, "test.log"))
	require.NoError(t, err)

	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &entry))
	assert.Equal(t, "oops error test", entry["msg"])
	assert.Equal(t, "DB_ERROR", entry["error.code"])
	assert.Equal(t, "database", entry["error.domain"])
	assert.Equal(t, "check connection", entry["error.hint"])
	assert.Equal(t, "extra_val", entry["extra_key"])
}

func TestLogger_ErrorContext_WithRegularError(t *testing.T) {
	dir := t.TempDir()
	cfg := log.Config{
		Level:  "debug",
		Format: "json",
		Mode:   "file",
		File: log.FileConfig{
			Dir:      dir,
			Filename: "test.log",
		},
	}
	l, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)
	defer func() { _ = l.Close() }()

	ctx := context.Background()
	l.ErrorContext(ctx, "regular error test", assert.AnError, "key", "val")

	data, err := os.ReadFile(filepath.Join(dir, "test.log"))
	require.NoError(t, err)

	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &entry))
	assert.Equal(t, "regular error test", entry["msg"])
	assert.Equal(t, "val", entry["key"])
	assert.Nil(t, entry["error.code"], "regular error should not have error.code")
}

func TestLogger_ErrorContext_NilError(t *testing.T) {
	dir := t.TempDir()
	cfg := log.Config{
		Level:  "debug",
		Format: "json",
		Mode:   "file",
		File: log.FileConfig{
			Dir:      dir,
			Filename: "test.log",
		},
	}
	l, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)
	defer func() { _ = l.Close() }()

	ctx := context.Background()
	l.ErrorContext(ctx, "nil error test", nil, "key", "val")

	data, err := os.ReadFile(filepath.Join(dir, "test.log"))
	require.NoError(t, err)

	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &entry))
	assert.Equal(t, "nil error test", entry["msg"])
	assert.Nil(t, entry["error.code"])
}

func TestNewLogger_WithCategory(t *testing.T) {
	dir := t.TempDir()
	cfg := log.Config{
		Level:  "info",
		Format: "json",
		Mode:   "file",
		File: log.FileConfig{
			Dir:      dir,
			Filename: "test.log",
		},
	}
	l, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)
	defer func() { _ = l.Close() }()

	accessLog := l.WithCategory(log.CategoryAccess)
	accessLog.InfoContext(context.Background(), "category test")

	data, err := os.ReadFile(filepath.Join(dir, "test.log"))
	require.NoError(t, err)

	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &entry))
	assert.Equal(t, "access", entry["category"])
}

func TestNewLogger_FileMode_LevelFiltering(t *testing.T) {
	dir := t.TempDir()
	cfg := log.Config{
		Level:  "warn",
		Format: "json",
		Mode:   "file",
		File: log.FileConfig{
			Dir:      dir,
			Filename: "test.log",
		},
	}
	l, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)
	defer func() { _ = l.Close() }()

	l.InfoContext(context.Background(), "should be filtered")
	l.WarnContext(context.Background(), "should appear")

	data, err := os.ReadFile(filepath.Join(dir, "test.log"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "should be filtered")
	assert.Contains(t, string(data), "should appear")
}

func TestNewLogger_CategoriesWarning(t *testing.T) {
	cfg := log.Config{
		Level:  "info",
		Format: "json",
		Mode:   "console",
		Categories: map[string]log.CategoryConfig{
			"access": {Enabled: true, File: "access.log"},
		},
	}
	l, err := log.NewLogger(cfg, log.ReleaseInfo{})
	require.NoError(t, err)
	require.NotNil(t, l)
}
