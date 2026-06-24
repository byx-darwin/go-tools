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

// ── Options 模式测试 ──

func TestNew_Defaults(t *testing.T) {
	l := New()
	assert.NotNil(t, l)
	assert.NotNil(t, l.Logger)
}

func TestNew_WithLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l := New(WithLevel("debug"), WithFilePath(path), WithJSON(true))
	defer func() { _ = l.Close() }()

	l.Debug("debug msg")
	data, _ := os.ReadFile(path)
	assert.Contains(t, string(data), "debug msg")
}

func TestNew_WithFilePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l := New(WithFilePath(path))
	defer func() { _ = l.Close() }()

	l.Info("hello")
	_, err := os.Stat(path)
	assert.NoError(t, err)
}

func TestNew_WithJSON_Text(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l := New(WithLevel("info"), WithFilePath(path), WithJSON(false))
	defer func() { _ = l.Close() }()

	l.Info("text msg")
	data, _ := os.ReadFile(path)
	assert.Contains(t, string(data), "text msg")
	// text 格式不是 JSON
	var entry map[string]interface{}
	err := json.Unmarshal(data, &entry)
	assert.Error(t, err, "text format should not be valid JSON")
}

func TestNew_WithJSON_Format(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l := New(WithLevel("info"), WithFilePath(path), WithJSON(true))
	defer func() { _ = l.Close() }()

	l.Info("hello", "key", "value")
	data, _ := os.ReadFile(path)

	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &entry))
	assert.Equal(t, "hello", entry["msg"])
	assert.Equal(t, "INFO", entry["level"])
	assert.Equal(t, "value", entry["key"])
}

func TestNew_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logs", "subdir", "app.log")

	l := New(WithFilePath(path))
	defer func() { _ = l.Close() }()

	l.Info("test")
	_, err := os.Stat(path)
	assert.NoError(t, err, "log file should be created with parent dirs")
}

func TestNew_InvalidLevelDefaultsToInfo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l := New(WithLevel("invalid"), WithFilePath(path))
	defer func() { _ = l.Close() }()

	l.Debug("should not appear")
	l.Info("should appear")

	data, _ := os.ReadFile(path)
	assert.Contains(t, string(data), "should appear")
	assert.NotContains(t, string(data), "should not appear")
}

// ── Config 模式测试（向后兼容） ──

func TestNewFromConfig_Defaults(t *testing.T) {
	l := NewFromConfig(Config{})
	assert.NotNil(t, l)
}

func TestNewFromConfig_JSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l := NewFromConfig(Config{Level: "info", FilePath: path, JSON: true})
	defer func() { _ = l.Close() }()

	l.Info("hello", "key", "value")
	data, _ := os.ReadFile(path)

	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &entry))
	assert.Equal(t, "hello", entry["msg"])
	assert.Equal(t, "INFO", entry["level"])
}

func TestNewFromConfig_Text(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l := NewFromConfig(Config{Level: "debug", FilePath: path, JSON: false})
	defer func() { _ = l.Close() }()

	l.Debug("debug message")
	data, _ := os.ReadFile(path)
	assert.Contains(t, string(data), "debug message")
}

func TestNewFromConfig_Stdout(t *testing.T) {
	l := NewFromConfig(Config{Level: "error"})
	assert.NotNil(t, l)
}

// ── 通用测试 ──

func TestLogger_LevelsDebug(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l := New(WithLevel("debug"), WithFilePath(path), WithJSON(true))
	defer func() { _ = l.Close() }()

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

	l := New(WithLevel("error"), WithFilePath(path), WithJSON(true))
	defer func() { _ = l.Close() }()

	l.Info("should be filtered")
	l.Error("should appear")

	data, _ := os.ReadFile(path)
	assert.NotContains(t, string(data), "should be filtered")
	assert.Contains(t, string(data), "should appear")
}

func TestLogger_WithContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l := New(WithLevel("info"), WithFilePath(path), WithJSON(true))
	defer func() { _ = l.Close() }()

	ctx := context.Background()
	l.InfoContext(ctx, "with context", "ctx_key", "ctx_val")

	data, _ := os.ReadFile(path)
	assert.Contains(t, string(data), "with context")
	assert.Contains(t, string(data), "ctx_key")
}

func TestLogger_WithAttrs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l := New(WithLevel("info"), WithFilePath(path), WithJSON(true))
	defer func() { _ = l.Close() }()

	child := l.With("component", "test")
	child.Info("from child logger")

	data, _ := os.ReadFile(path)
	assert.Contains(t, string(data), "from child logger")
	assert.Contains(t, string(data), "component")
	assert.Contains(t, string(data), "test")
}

func TestLogger_Close(t *testing.T) {
	l := New() // stdout, no writer to close
	assert.NoError(t, l.Close())
}

func TestParseLevel(t *testing.T) {
	assert.Equal(t, slog.LevelDebug, parseLevel("debug"))
	assert.Equal(t, slog.LevelInfo, parseLevel("info"))
	assert.Equal(t, slog.LevelWarn, parseLevel("warn"))
	assert.Equal(t, slog.LevelError, parseLevel("error"))
	assert.Equal(t, slog.LevelInfo, parseLevel("unknown"))
}

func TestConfig_DefaultsApplied(t *testing.T) {
	l := NewFromConfig(Config{})
	assert.Equal(t, 100, l.config.MaxSize)
	assert.Equal(t, 7, l.config.MaxBackups)
	assert.Equal(t, 30, l.config.MaxAge)
}

func TestOption_ZeroValuesIgnored(t *testing.T) {
	l := New(
		WithLevel(""),
		WithMaxSize(0),
		WithMaxBackups(0),
		WithMaxAge(0),
	)
	// 零值不应覆盖默认值
	assert.Equal(t, defaultLevel, l.config.Level)
	assert.Equal(t, defaultMaxSize, l.config.MaxSize)
}
