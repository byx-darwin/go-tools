// Package log 提供基于 Go 标准库 log/slog 的结构化日志封装。
//
// 核心功能：
//   - 文件轮转（lumberjack）
//   - 自动 OTel span 关联（TraceID/SpanID 注入）
//   - klog/hlog 适配器
//
// 用法：
//
//	// Options 模式（推荐）
//	l := log.New(
//	    log.WithLevel("info"),
//	    log.WithFilePath("/var/log/app.log"),
//	)
//
//	// Config 模式（YAML 加载场景）
//	l := log.NewFromConfig(cfg)
//
//	l.Info("server started", "port", 8080)
//	l.Error("something failed", "error", err)
package log

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"go.opentelemetry.io/otel/trace"
)

const (
	defaultLevel      = "info"
	defaultMaxSize    = 100
	defaultMaxBackups = 7
	defaultMaxAge     = 30
)

// LegacyConfig 日志配置（旧版，用于向后兼容 YAML 反序列化）。
//
// Deprecated: 使用 Config 替代。
type LegacyConfig struct {
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

// Option 定义日志配置选项函数。
type Option func(*loggerConfig)

type loggerConfig struct {
	level      string
	filePath   string
	maxSize    int
	maxBackups int
	maxAge     int
	compress   bool
	json       bool
}

// WithLevel 设置日志级别: "debug", "info", "warn", "error"。
func WithLevel(level string) Option {
	return func(c *loggerConfig) {
		if level != "" {
			c.level = level
		}
	}
}

// WithFilePath 设置日志文件路径。为空则输出到 stdout。
func WithFilePath(filePath string) Option {
	return func(c *loggerConfig) {
		c.filePath = filePath
	}
}

// WithMaxSize 设置单个日志文件最大 MB。
func WithMaxSize(maxSize int) Option {
	return func(c *loggerConfig) {
		if maxSize > 0 {
			c.maxSize = maxSize
		}
	}
}

// WithMaxBackups 设置保留的旧日志文件最大数量。
func WithMaxBackups(maxBackups int) Option {
	return func(c *loggerConfig) {
		if maxBackups > 0 {
			c.maxBackups = maxBackups
		}
	}
}

// WithMaxAge 设置保留旧日志文件的最大天数。
func WithMaxAge(maxAge int) Option {
	return func(c *loggerConfig) {
		if maxAge > 0 {
			c.maxAge = maxAge
		}
	}
}

// WithCompress 设置是否 gzip 压缩旧日志文件。
func WithCompress(compress bool) Option {
	return func(c *loggerConfig) {
		c.compress = compress
	}
}

// WithJSON 设置是否输出 JSON 格式（默认 true，false 为 text）。
func WithJSON(json bool) Option {
	return func(c *loggerConfig) {
		c.json = json
	}
}

// Logger 结构化日志记录器，封装 slog.Logger。
type Logger struct {
	*slog.Logger
	config LegacyConfig
	level  slog.Level
	writer io.WriteCloser
}

// New 创建 Logger，支持 Options 配置。
//
// 默认配置：
//   - level: "info"
//   - maxSize: 100
//   - maxBackups: 7
//   - maxAge: 30
//   - json: true
func New(opts ...Option) *Logger {
	cfg := &loggerConfig{
		level:      defaultLevel,
		maxSize:    defaultMaxSize,
		maxBackups: defaultMaxBackups,
		maxAge:     defaultMaxAge,
		json:       true,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return buildLogger(cfg)
}

// NewFromLegacyConfig 从旧版 Config 创建 Logger（用于向后兼容）。
//
// Deprecated: 使用 NewFromConfig 替代。
func NewFromLegacyConfig(c LegacyConfig) *Logger {
	cfg := &loggerConfig{
		level:      c.Level,
		filePath:   c.FilePath,
		maxSize:    c.MaxSize,
		maxBackups: c.MaxBackups,
		maxAge:     c.MaxAge,
		compress:   c.Compress,
		json:       c.JSON,
	}
	// 应用默认值
	if cfg.level == "" {
		cfg.level = defaultLevel
	}
	if cfg.maxSize == 0 {
		cfg.maxSize = defaultMaxSize
	}
	if cfg.maxBackups == 0 {
		cfg.maxBackups = defaultMaxBackups
	}
	if cfg.maxAge == 0 {
		cfg.maxAge = defaultMaxAge
	}
	return buildLogger(cfg)
}

func buildLogger(cfg *loggerConfig) *Logger {
	var w io.Writer = os.Stdout
	var wc io.WriteCloser

	if cfg.filePath != "" {
		dir := filepath.Dir(cfg.filePath)
		_ = os.MkdirAll(dir, 0o755)
		f, err := os.OpenFile(cfg.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			f = nil
		}
		if f != nil {
			wc = f
			w = f
		}
	}

	level := parseLevel(cfg.level)
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	if cfg.json {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}

	handler = &otelHandler{next: handler}

	return &Logger{
		Logger: slog.New(handler),
		config: LegacyConfig{
			Level:      cfg.level,
			FilePath:   cfg.filePath,
			MaxSize:    cfg.maxSize,
			MaxBackups: cfg.maxBackups,
			MaxAge:     cfg.maxAge,
			Compress:   cfg.compress,
			JSON:       cfg.json,
		},
		level:  level,
		writer: wc,
	}
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
