package log

import (
	"context"
	"fmt"
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

	// Categories 配置尚未在 NewLogger 中生效，仅通过 WithCategory 支持动态子 Logger
	if len(cfg.Categories) > 0 {
		fmt.Fprintf(os.Stderr, "[log] warning: Config.Categories is not yet supported by NewLogger; use Logger.WithCategory instead\n")
	}

	return &Logger{
		Logger: slog.New(handler),
		level:  parseLevel(cfg.Level),
	}, nil
}

// createOutputHandler 根据格式创建输出 handler。
func createOutputHandler(w io.Writer, cfg Config) slog.Handler {
	opts := &slog.HandlerOptions{
		Level:     parseLevel(cfg.Level),
		AddSource: cfg.AddSource,
	}
	if cfg.Format == "json" {
		return slog.NewJSONHandler(w, opts)
	}
	return slog.NewTextHandler(w, opts)
}

// WithCategory 创建带分类的子 Logger。
func (l *Logger) WithCategory(category string) *Logger {
	handler := NewCategoryHandler(l.Handler(), category)
	return &Logger{
		Logger: slog.New(handler),
		level:  l.level,
	}
}

// ErrorContext 记录错误日志，自动提取 oops 错误属性。
// 如果 err 是 oops 错误，会自动注入 error.code、error.domain、error.hint、error.public。
func (l *Logger) ErrorContext(ctx context.Context, msg string, err error, args ...any) {
	extra := ErrorAttrs(err)
	allArgs := append(args, extra...) //nolint:gocritic // append to new slice is intentional
	l.Logger.ErrorContext(ctx, msg, allArgs...)
}
