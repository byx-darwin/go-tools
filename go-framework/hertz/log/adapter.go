// Package hertzlog 提供 go-common/log.Logger 到 Hertz hlog.FullLogger 的适配器。
package hertzlog

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/byx-darwin/go-tools/go-common/log"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// HertzAdapter 将 go-common/log.Logger 适配为 hertz hlog.FullLogger。
type HertzAdapter struct {
	logger *log.Logger
	level  hlog.Level
	writer io.Writer
}

// NewHertzAdapter 创建 Hertz 日志适配器。
func NewHertzAdapter(l *log.Logger) hlog.FullLogger {
	return &HertzAdapter{
		logger: l,
		level:  hlog.LevelInfo,
	}
}

// ── Logger interface ──

// Trace implements hlog.Logger.
func (a *HertzAdapter) Trace(v ...interface{}) { a.log(slog.LevelDebug, fmt.Sprint(v...)) }

// Debug implements hlog.Logger.
func (a *HertzAdapter) Debug(v ...interface{}) { a.log(slog.LevelDebug, fmt.Sprint(v...)) }

// Info implements hlog.Logger.
func (a *HertzAdapter) Info(v ...interface{}) { a.log(slog.LevelInfo, fmt.Sprint(v...)) }

// Notice implements hlog.Logger.
func (a *HertzAdapter) Notice(v ...interface{}) { a.log(slog.LevelInfo, fmt.Sprint(v...)) }

// Warn implements hlog.Logger.
func (a *HertzAdapter) Warn(v ...interface{}) { a.log(slog.LevelWarn, fmt.Sprint(v...)) }

// Error implements hlog.Logger.
func (a *HertzAdapter) Error(v ...interface{}) { a.log(slog.LevelError, fmt.Sprint(v...)) }

// Fatal implements hlog.Logger.
func (a *HertzAdapter) Fatal(v ...interface{}) { a.log(slog.LevelError, fmt.Sprint(v...)) }

// ── FormatLogger interface ──

// Tracef implements hlog.FormatLogger.
func (a *HertzAdapter) Tracef(format string, v ...interface{}) {
	a.log(slog.LevelDebug, fmt.Sprintf(format, v...))
}

// Debugf implements hlog.FormatLogger.
func (a *HertzAdapter) Debugf(format string, v ...interface{}) {
	a.log(slog.LevelDebug, fmt.Sprintf(format, v...))
}

// Infof implements hlog.FormatLogger.
func (a *HertzAdapter) Infof(format string, v ...interface{}) {
	a.log(slog.LevelInfo, fmt.Sprintf(format, v...))
}

// Noticef implements hlog.FormatLogger.
func (a *HertzAdapter) Noticef(format string, v ...interface{}) {
	a.log(slog.LevelInfo, fmt.Sprintf(format, v...))
}

// Warnf implements hlog.FormatLogger.
func (a *HertzAdapter) Warnf(format string, v ...interface{}) {
	a.log(slog.LevelWarn, fmt.Sprintf(format, v...))
}

// Errorf implements hlog.FormatLogger.
func (a *HertzAdapter) Errorf(format string, v ...interface{}) {
	a.log(slog.LevelError, fmt.Sprintf(format, v...))
}

// Fatalf implements hlog.FormatLogger.
func (a *HertzAdapter) Fatalf(format string, v ...interface{}) {
	a.log(slog.LevelError, fmt.Sprintf(format, v...))
}

// ── CtxLogger interface ──

// CtxTracef implements hlog.CtxLogger.
func (a *HertzAdapter) CtxTracef(ctx context.Context, format string, v ...interface{}) {
	a.logger.DebugContext(ctx, fmt.Sprintf(format, v...))
}

// CtxDebugf implements hlog.CtxLogger.
func (a *HertzAdapter) CtxDebugf(ctx context.Context, format string, v ...interface{}) {
	a.logger.DebugContext(ctx, fmt.Sprintf(format, v...))
}

// CtxInfof implements hlog.CtxLogger.
func (a *HertzAdapter) CtxInfof(ctx context.Context, format string, v ...interface{}) {
	a.logger.InfoContext(ctx, fmt.Sprintf(format, v...))
}

// CtxNoticef implements hlog.CtxLogger.
func (a *HertzAdapter) CtxNoticef(ctx context.Context, format string, v ...interface{}) {
	a.logger.InfoContext(ctx, fmt.Sprintf(format, v...))
}

// CtxWarnf implements hlog.CtxLogger.
func (a *HertzAdapter) CtxWarnf(ctx context.Context, format string, v ...interface{}) {
	a.logger.WarnContext(ctx, fmt.Sprintf(format, v...))
}

// CtxErrorf implements hlog.CtxLogger.
func (a *HertzAdapter) CtxErrorf(ctx context.Context, format string, v ...interface{}) {
	a.logger.Logger.ErrorContext(ctx, fmt.Sprintf(format, v...))
}

// CtxFatalf implements hlog.CtxLogger.
func (a *HertzAdapter) CtxFatalf(ctx context.Context, format string, v ...interface{}) {
	a.logger.Logger.ErrorContext(ctx, fmt.Sprintf(format, v...))
}

// ── Control interface ──

// SetLevel implements hlog.ControlLogger.
func (a *HertzAdapter) SetLevel(level hlog.Level) { a.level = level }

// SetOutput implements hlog.ControlLogger.
func (a *HertzAdapter) SetOutput(w io.Writer) { a.writer = w }

func (a *HertzAdapter) log(level slog.Level, msg string) {
	a.logger.Logger.LogAttrs(context.Background(), level, msg)
}
