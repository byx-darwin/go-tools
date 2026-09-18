package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
)

// loggerBuildOptions 收集 NewLogger 构建过程的可选项。
type loggerBuildOptions struct {
	extraHandlers []func(slog.Handler) slog.Handler
}

// LoggerOption 配置 NewLogger 构建过程的可选项。
type LoggerOption func(*loggerBuildOptions)

// WithExtraHandler 注入一个自定义 handler 包装函数，应用在标准 handler 链
// （output → context → release → mask）构建完成之后，作为最外层 handler。
// 可多次调用，按调用顺序层层包装（最后一次调用的在最外层，最先执行）。
func WithExtraHandler(wrap func(slog.Handler) slog.Handler) LoggerOption {
	return func(o *loggerBuildOptions) {
		if wrap != nil {
			o.extraHandlers = append(o.extraHandlers, wrap)
		}
	}
}

// NewLogger 使用新 Config 和 ReleaseInfo 创建 Logger，支持可选的 LoggerOption。
func NewLogger(cfg Config, release ReleaseInfo, opts ...LoggerOption) (*Logger, error) {
	o := &loggerBuildOptions{}
	for _, opt := range opts {
		opt(o)
	}

	var handler slog.Handler

	// 创建输出 handler
	var outputHandler slog.Handler
	switch cfg.Mode {
	case "file":
		if cfg.File.Dir == "" || cfg.File.Filename == "" {
			fmt.Fprintf(os.Stderr, "[log] warning: mode=file but File.Dir/Filename is empty; falling back to console\n")
			outputHandler = createOutputHandler(os.Stdout, cfg)
		} else {
			w := createFileWriter(cfg.File)
			outputHandler = createOutputHandler(w, cfg)
		}
	case "both":
		if cfg.File.Dir == "" || cfg.File.Filename == "" {
			fmt.Fprintf(os.Stderr, "[log] warning: mode=both but File.Dir/Filename is empty; falling back to console (single output)\n")
			outputHandler = createOutputHandler(os.Stdout, cfg)
		} else {
			w := createFileWriter(cfg.File)
			fileHandler := createOutputHandler(w, cfg)
			consoleHandler := createOutputHandler(os.Stdout, cfg)
			outputHandler = NewMultiHandler(consoleHandler, fileHandler)
		}
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

	for _, wrap := range o.extraHandlers {
		handler = wrap(handler)
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
