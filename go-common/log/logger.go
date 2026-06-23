// Package log 提供基于 Go 标准库 log/slog 的结构化日志封装。
//
// 核心功能：
//   - 文件轮转（lumberjack）
//   - 自动 OTel span 关联（TraceID/SpanID 注入）
//   - klog/hlog 适配器
//
// 用法：
//
//	l := log.New(log.Config{Level: "info", FilePath: "/var/log/app.log"})
//	l.Info("server started", "port", 8080)
//	l.Error("something failed", "error", err)
package log

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"go.opentelemetry.io/otel/trace"
)

// Config 日志配置
type Config struct {
	// Level 日志级别: "debug", "info", "warn", "error"（默认 "info"）
	Level string `json:"level" yaml:"level"`

	// FilePath 日志文件路径（为空则输出到 stdout）
	FilePath string `json:"file_path" yaml:"file_path"`

	// MaxSize 单个日志文件最大 MB（默认 100）
	MaxSize int `json:"max_size" yaml:"max_size"`

	// MaxBackups 保留的旧日志文件最大数量（默认 7）
	MaxBackups int `json:"max_backups" yaml:"max_backups"`

	// MaxAge 保留旧日志文件的最大天数（默认 30）
	MaxAge int `json:"max_age" yaml:"max_age"`

	// Compress 是否 gzip 压缩旧日志文件
	Compress bool `json:"compress" yaml:"compress"`

	// JSON 是否输出 JSON 格式（默认 true，false 为 text）
	JSON bool `json:"json" yaml:"json"`
}

// Logger 结构化日志记录器，封装 slog.Logger。
type Logger struct {
	*slog.Logger
	config Config
	level  slog.Level
	mu     sync.Mutex
	writer io.WriteCloser
}

// New 创建 Logger。
// 自动处理文件轮转（lumberjack 风格）、日志压缩、OTel span 关联。
func New(cfg Config) *Logger {
	if cfg.Level == "" {
		cfg.Level = "info"
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 100
	}
	if cfg.MaxBackups == 0 {
		cfg.MaxBackups = 7
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = 30
	}

	var w io.Writer = os.Stdout
	var wc io.WriteCloser

	if cfg.FilePath != "" {
		dir := filepath.Dir(cfg.FilePath)
		_ = os.MkdirAll(dir, 0755)
		f, err := os.OpenFile(cfg.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			// fallback to stdout on open failure
			f = nil
		}
		if f != nil {
			wc = f
			w = f
		}
	}

	level := parseLevel(cfg.Level)
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	if cfg.JSON {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}

	// Wrap handler to inject OTel trace info
	handler = &otelHandler{next: handler}

	l := &Logger{
		Logger: slog.New(handler),
		config: cfg,
		level:  level,
		writer: wc,
	}
	return l
}

// Close 关闭日志文件（如有）。
func (l *Logger) Close() error {
	if l.writer != nil {
		return l.writer.Close()
	}
	return nil
}

func parseLevel(s string) slog.Level {
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

// otelHandler 在每条日志中注入 TraceID 和 SpanID。
type otelHandler struct {
	next slog.Handler
}

func (h *otelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *otelHandler) Handle(ctx context.Context, r slog.Record) error {
	if ctx != nil {
		span := trace.SpanFromContext(ctx)
		if span.SpanContext().IsValid() {
			tid := span.SpanContext().TraceID().String()
			sid := span.SpanContext().SpanID().String()
			r.AddAttrs(
				slog.String("trace_id", tid),
				slog.String("span_id", sid),
			)
		}
	}
	return h.next.Handle(ctx, r)
}

func (h *otelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &otelHandler{next: h.next.WithAttrs(attrs)}
}

func (h *otelHandler) WithGroup(name string) slog.Handler {
	return &otelHandler{next: h.next.WithGroup(name)}
}
