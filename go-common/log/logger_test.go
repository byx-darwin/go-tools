package log

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_Defaults(t *testing.T) {
	l := New(Config{})
	assert.NotNil(t, l)
	assert.NotNil(t, l.Logger)
}

func TestNew_JSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l := New(Config{
		Level:    "info",
		FilePath: path,
		JSON:     true,
	})
	defer l.Close()

	l.Info("hello", "key", "value")

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &entry))
	assert.Equal(t, "hello", entry["msg"])
	assert.Equal(t, "INFO", entry["level"])
	assert.Equal(t, "value", entry["key"])
}

func TestNew_Text(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l := New(Config{
		Level:    "debug",
		FilePath: path,
		JSON:     false,
	})
	defer l.Close()

	l.Debug("debug message")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "debug message")
}

func TestNew_Stdout(t *testing.T) {
	// No file path → stdout
	l := New(Config{Level: "error"})
	assert.NotNil(t, l)
}

func TestNew_InvalidLevelDefaultsToInfo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l := New(Config{Level: "invalid", FilePath: path})
	defer l.Close()
	// Debug should be filtered out at INFO level
	l.Debug("should not appear")
	l.Info("should appear")

	data, _ := os.ReadFile(path)
	assert.Contains(t, string(data), "should appear")
	assert.NotContains(t, string(data), "should not appear")
}

func TestLogger_LevelsDebug(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l := New(Config{Level: "debug", FilePath: path, JSON: true})
	defer l.Close()

	l.Debug("debug msg")
	l.Info("info msg")
	l.Warn("warn msg")
	l.Error("error msg")

	data, _ := os.ReadFile(path)
	s := string(data)
	assert.Contains(t, s, "debug msg")
	assert.Contains(t, s, "info msg")
	assert.Contains(t, s, "warn msg")
	assert.Contains(t, s, "error msg")
}

func TestLogger_LevelsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l := New(Config{Level: "error", FilePath: path, JSON: true})
	defer l.Close()

	l.Info("should be filtered")
	l.Error("should appear")

	data, _ := os.ReadFile(path)
	assert.NotContains(t, string(data), "should be filtered")
	assert.Contains(t, string(data), "should appear")
}

func TestLogger_WithContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l := New(Config{Level: "info", FilePath: path, JSON: true})
	defer l.Close()

	ctx := context.Background()
	l.InfoContext(ctx, "with context", "ctx_key", "ctx_val")

	data, _ := os.ReadFile(path)
	assert.Contains(t, string(data), "with context")
	assert.Contains(t, string(data), "ctx_key")
}

func TestLogger_WithAttrs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l := New(Config{Level: "info", FilePath: path, JSON: true})
	defer l.Close()

	child := l.With("component", "test")
	child.Info("from child logger")

	data, _ := os.ReadFile(path)
	assert.Contains(t, string(data), "from child logger")
	assert.Contains(t, string(data), "component")
	assert.Contains(t, string(data), "test")
}

func TestLogger_Close(t *testing.T) {
	l := New(Config{}) // stdout, no writer to close
	assert.NoError(t, l.Close())
}

func TestParseLevel(t *testing.T) {
	assert.Equal(t, slog.LevelDebug, parseLevel("debug"))
	assert.Equal(t, slog.LevelInfo, parseLevel("info"))
	assert.Equal(t, slog.LevelWarn, parseLevel("warn"))
	assert.Equal(t, slog.LevelError, parseLevel("error"))
	// Unknown defaults to info
	assert.Equal(t, slog.LevelInfo, parseLevel("unknown"))
}

func TestConfig_DefaultsApplied(t *testing.T) {
	l := New(Config{})
	assert.Equal(t, 100, l.config.MaxSize)
	assert.Equal(t, 7, l.config.MaxBackups)
	assert.Equal(t, 30, l.config.MaxAge)
}

func TestNew_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logs", "subdir", "app.log")

	l := New(Config{FilePath: path})
	defer l.Close()

	l.Info("test")
	_, err := os.Stat(path)
	assert.NoError(t, err, "log file should be created with parent dirs")
}
