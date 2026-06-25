package log

import (
	"io"
	"log/slog"
	"os"
)

// NewLogger 使用新 Config 和 ReleaseInfo 创建 Logger。
func NewLogger(cfg Config, release ReleaseInfo) (*Logger, error) {
	var handler slog.Handler

	// 创建输出 handler
	var outputHandler slog.Handler
	switch cfg.Mode {
	case "file":
		w := createFileWriter(cfg.File)
		outputHandler = createOutputHandler(w, cfg)
	case "both":
		w := createFileWriter(cfg.File)
		fileHandler := createOutputHandler(w, cfg)
		consoleHandler := createOutputHandler(os.Stdout, cfg)
		outputHandler = NewMultiHandler(consoleHandler, fileHandler)
	default: // "console"
		outputHandler = createOutputHandler(os.Stdout, cfg)
	}

	// 构建 handler 链
	handler = outputHandler

	// 添加 context handler
	handler = NewContextHandler(handler)

	// 添加 release handler
	handler = NewReleaseHandler(handler, release)

	// 添加 mask handler
	if cfg.Masking.Enabled {
		masker := NewMasker(cfg.Masking)
		handler = NewMaskHandler(handler, masker)
	}

	return &Logger{
		Logger: slog.New(handler),
		level:  parseNewLevel(cfg.Level),
	}, nil
}

// createOutputHandler 根据格式创建输出 handler。
func createOutputHandler(w io.Writer, cfg Config) slog.Handler {
	opts := &slog.HandlerOptions{
		Level:     parseNewLevel(cfg.Level),
		AddSource: cfg.AddSource,
	}
	if cfg.Format == "json" {
		return slog.NewJSONHandler(w, opts)
	}
	return slog.NewTextHandler(w, opts)
}

// parseNewLevel 解析日志级别字符串。
func parseNewLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// WithCategory 创建带分类的子 Logger。
func (l *Logger) WithCategory(category string) *Logger {
	handler := NewCategoryHandler(l.Logger.Handler(), category)
	return &Logger{
		Logger: slog.New(handler),
		level:  l.level,
	}
}
