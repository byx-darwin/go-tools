// Package kitexlog 提供 go-common/log.Logger 到 Kitex klog.FullLogger 的适配器。
package kitexlog

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/byx-darwin/go-tools/go-common/log"
	"github.com/cloudwego/kitex/pkg/klog"
)

// KitexAdapter 将 go-common/log.Logger 适配为 kitex klog.FullLogger。
type KitexAdapter struct {
	logger *log.Logger
	level  klog.Level
	writer io.Writer
}

// NewKitexAdapter 创建 Kitex 日志适配器。
func NewKitexAdapter(l *log.Logger) klog.FullLogger {
	return &KitexAdapter{
		logger: l,
		level:  klog.LevelInfo,
	}
}

// ── Logger interface ──

// Trace implements klog.Logger.
func (a *KitexAdapter) Trace(v ...interface{}) { a.log(slog.LevelDebug, fmt.Sprint(v...)) }

// Debug implements klog.Logger.
func (a *KitexAdapter) Debug(v ...interface{}) { a.log(slog.LevelDebug, fmt.Sprint(v...)) }

// Info implements klog.Logger.
func (a *KitexAdapter) Info(v ...interface{}) { a.log(slog.LevelInfo, fmt.Sprint(v...)) }

// Notice implements klog.Logger.
func (a *KitexAdapter) Notice(v ...interface{}) { a.log(slog.LevelInfo, fmt.Sprint(v...)) }

// Warn implements klog.Logger.
func (a *KitexAdapter) Warn(v ...interface{}) { a.log(slog.LevelWarn, fmt.Sprint(v...)) }

// Error implements klog.Logger.
func (a *KitexAdapter) Error(v ...interface{}) { a.log(slog.LevelError, fmt.Sprint(v...)) }

// Fatal implements klog.Logger.
func (a *KitexAdapter) Fatal(v ...interface{}) { a.log(slog.LevelError, fmt.Sprint(v...)) }

// ── FormatLogger interface ──

// Tracef implements klog.FormatLogger.
func (a *KitexAdapter) Tracef(format string, v ...interface{}) {
	a.log(slog.LevelDebug, fmt.Sprintf(format, v...))
}

// Debugf implements klog.FormatLogger.
func (a *KitexAdapter) Debugf(format string, v ...interface{}) {
	a.log(slog.LevelDebug, fmt.Sprintf(format, v...))
}

// Infof implements klog.FormatLogger.
func (a *KitexAdapter) Infof(format string, v ...interface{}) {
	a.log(slog.LevelInfo, fmt.Sprintf(format, v...))
}

// Noticef implements klog.FormatLogger.
func (a *KitexAdapter) Noticef(format string, v ...interface{}) {
	a.log(slog.LevelInfo, fmt.Sprintf(format, v...))
}

// Warnf implements klog.FormatLogger.
func (a *KitexAdapter) Warnf(format string, v ...interface{}) {
	a.log(slog.LevelWarn, fmt.Sprintf(format, v...))
}

// Errorf implements klog.FormatLogger.
func (a *KitexAdapter) Errorf(format string, v ...interface{}) {
	a.log(slog.LevelError, fmt.Sprintf(format, v...))
}

// Fatalf implements klog.FormatLogger.
func (a *KitexAdapter) Fatalf(format string, v ...interface{}) {
	a.log(slog.LevelError, fmt.Sprintf(format, v...))
}

// ── CtxLogger interface ──

// CtxTracef implements klog.CtxLogger.
func (a *KitexAdapter) CtxTracef(ctx context.Context, format string, v ...interface{}) {
	a.logger.DebugContext(ctx, fmt.Sprintf(format, v...))
}

// CtxDebugf implements klog.CtxLogger.
func (a *KitexAdapter) CtxDebugf(ctx context.Context, format string, v ...interface{}) {
	a.logger.DebugContext(ctx, fmt.Sprintf(format, v...))
}

// CtxInfof implements klog.CtxLogger.
func (a *KitexAdapter) CtxInfof(ctx context.Context, format string, v ...interface{}) {
	a.logger.InfoContext(ctx, fmt.Sprintf(format, v...))
}

// CtxNoticef implements klog.CtxLogger.
func (a *KitexAdapter) CtxNoticef(ctx context.Context, format string, v ...interface{}) {
	a.logger.InfoContext(ctx, fmt.Sprintf(format, v...))
}

// CtxWarnf implements klog.CtxLogger.
func (a *KitexAdapter) CtxWarnf(ctx context.Context, format string, v ...interface{}) {
	a.logger.WarnContext(ctx, fmt.Sprintf(format, v...))
}

// CtxErrorf implements klog.CtxLogger.
func (a *KitexAdapter) CtxErrorf(ctx context.Context, format string, v ...interface{}) {
	a.logger.ErrorContext(ctx, fmt.Sprintf(format, v...))
}

// CtxFatalf implements klog.CtxLogger.
func (a *KitexAdapter) CtxFatalf(ctx context.Context, format string, v ...interface{}) {
	a.logger.ErrorContext(ctx, fmt.Sprintf(format, v...))
}

// ── Control interface ──

// SetLevel implements klog.ControlLogger.
func (a *KitexAdapter) SetLevel(level klog.Level) { a.level = level }

// SetOutput implements klog.ControlLogger.
func (a *KitexAdapter) SetOutput(w io.Writer) { a.writer = w }

func (a *KitexAdapter) log(level slog.Level, msg string) {
	a.logger.Logger.LogAttrs(context.Background(), level, msg)
}
