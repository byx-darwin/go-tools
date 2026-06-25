package log_test

import (
	"testing"

	"github.com/byx-darwin/go-tools/go-common/log"
	"github.com/stretchr/testify/require"
)

func TestConfig_Defaults(t *testing.T) {
	cfg := log.NewConfig()
	require.Equal(t, "info", cfg.Level)
	require.Equal(t, "json", cfg.Format)
	require.Equal(t, "console", cfg.Mode)
	require.False(t, cfg.AddSource)
	require.Empty(t, cfg.Categories)
	require.False(t, cfg.Masking.Enabled)
}

func TestConfig_CustomValues(t *testing.T) {
	cfg := log.NewConfig(
		log.WithConfigLevel("debug"),
		log.WithConfigFormat("text"),
		log.WithConfigMode("file"),
		log.WithConfigAddSource(true),
	)
	require.Equal(t, "debug", cfg.Level)
	require.Equal(t, "text", cfg.Format)
	require.Equal(t, "file", cfg.Mode)
	require.True(t, cfg.AddSource)
}

func TestFileConfig_Defaults(t *testing.T) {
	cfg := log.NewFileConfig()
	require.Equal(t, 100, cfg.MaxSize)
	require.Equal(t, 7, cfg.MaxBackups)
	require.Equal(t, 30, cfg.MaxAge)
	require.Empty(t, cfg.Dir)
	require.Empty(t, cfg.Filename)
	require.False(t, cfg.Compress)
}

func TestFileConfig_CustomValues(t *testing.T) {
	cfg := log.NewFileConfig(
		log.WithFileDir("/var/log"),
		log.WithFilename("app.log"),
		log.WithFileMaxSize(200),
		log.WithFileMaxBackups(10),
		log.WithFileMaxAge(60),
		log.WithFileCompress(true),
	)
	require.Equal(t, "/var/log", cfg.Dir)
	require.Equal(t, "app.log", cfg.Filename)
	require.Equal(t, 200, cfg.MaxSize)
	require.Equal(t, 10, cfg.MaxBackups)
	require.Equal(t, 60, cfg.MaxAge)
	require.True(t, cfg.Compress)
}

func TestFileConfig_ZeroValuesIgnored(t *testing.T) {
	cfg := log.NewFileConfig(
		log.WithFileMaxSize(0),
		log.WithFileMaxBackups(0),
		log.WithFileMaxAge(0),
	)
	// 零值不应覆盖默认值
	require.Equal(t, 100, cfg.MaxSize)
	require.Equal(t, 7, cfg.MaxBackups)
	require.Equal(t, 30, cfg.MaxAge)
}

func TestCategoryConfig_Fields(t *testing.T) {
	cfg := log.CategoryConfig{
		Enabled: true,
		File:    "biz.log",
		Level:   "debug",
	}
	require.True(t, cfg.Enabled)
	require.Equal(t, "biz.log", cfg.File)
	require.Equal(t, "debug", cfg.Level)
}

func TestMaskConfig_Fields(t *testing.T) {
	cfg := log.MaskConfig{
		Enabled:      true,
		MaskedFields: []string{"password", "token"},
		Mode:         "partial",
	}
	require.True(t, cfg.Enabled)
	require.Equal(t, []string{"password", "token"}, cfg.MaskedFields)
	require.Equal(t, "partial", cfg.Mode)
}

func TestConfig_WithCategories(t *testing.T) {
	cfg := log.NewConfig(
		log.WithConfigCategories(map[string]log.CategoryConfig{
			"biz": {Enabled: true, File: "biz.log", Level: "debug"},
		}),
	)
	require.Len(t, cfg.Categories, 1)
	require.True(t, cfg.Categories["biz"].Enabled)
	require.Equal(t, "biz.log", cfg.Categories["biz"].File)
}

func TestConfig_WithMasking(t *testing.T) {
	mask := log.MaskConfig{
		Enabled:      true,
		MaskedFields: []string{"password"},
		Mode:         "full",
	}
	cfg := log.NewConfig(
		log.WithConfigMasking(mask),
	)
	require.True(t, cfg.Masking.Enabled)
	require.Equal(t, []string{"password"}, cfg.Masking.MaskedFields)
	require.Equal(t, "full", cfg.Masking.Mode)
}

func TestConfig_WithFile(t *testing.T) {
	fileCfg := log.NewFileConfig(
		log.WithFileDir("/var/log"),
		log.WithFilename("app.log"),
	)
	cfg := log.NewConfig(
		log.WithConfigFile(fileCfg),
	)
	require.Equal(t, "/var/log", cfg.File.Dir)
	require.Equal(t, "app.log", cfg.File.Filename)
	require.Equal(t, 100, cfg.File.MaxSize) // 默认值保留
}
