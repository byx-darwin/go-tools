// Package adapters 提供 slog.Logger 到框架日志接口的适配器。
package adapters

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/byx-darwin/go-tools/go-common/log"
	"github.com/cloudwego/kitex/pkg/klog"
)

// KitexAdapter 将 go-common/log.Logger 适配为 kitex klog.FullLogger。
//
// 用法:
//
//	l := log.New(log.WithLevel("info"))
//	klog.SetLogger(adapters.NewKitexAdapter(l))
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
		writer: osStdout{},
	}
}

// ── Logger interface ──

func (a *KitexAdapter) Trace(v ...interface{})  { a.log(slog.LevelDebug, fmt.Sprint(v...)) }
func (a *KitexAdapter) Debug(v ...interface{})  { a.log(slog.LevelDebug, fmt.Sprint(v...)) }
func (a *KitexAdapter) Info(v ...interface{})   { a.log(slog.LevelInfo, fmt.Sprint(v...)) }
func (a *KitexAdapter) Notice(v ...interface{}) { a.log(slog.LevelInfo, fmt.Sprint(v...)) }
func (a *KitexAdapter) Warn(v ...interface{})   { a.log(slog.LevelWarn, fmt.Sprint(v...)) }
func (a *KitexAdapter) Error(v ...interface{})  { a.log(slog.LevelError, fmt.Sprint(v...)) }
func (a *KitexAdapter) Fatal(v ...interface{})  { a.log(slog.LevelError, fmt.Sprint(v...)) }

// ── FormatLogger interface ──

func (a *KitexAdapter) Tracef(format string, v ...interface{}) {
	a.log(slog.LevelDebug, fmt.Sprintf(format, v...))
}
func (a *KitexAdapter) Debugf(format string, v ...interface{}) {
	a.log(slog.LevelDebug, fmt.Sprintf(format, v...))
}
func (a *KitexAdapter) Infof(format string, v ...interface{}) {
	a.log(slog.LevelInfo, fmt.Sprintf(format, v...))
}
func (a *KitexAdapter) Noticef(format string, v ...interface{}) {
	a.log(slog.LevelInfo, fmt.Sprintf(format, v...))
}
func (a *KitexAdapter) Warnf(format string, v ...interface{}) {
	a.log(slog.LevelWarn, fmt.Sprintf(format, v...))
}
func (a *KitexAdapter) Errorf(format string, v ...interface{}) {
	a.log(slog.LevelError, fmt.Sprintf(format, v...))
}
func (a *KitexAdapter) Fatalf(format string, v ...interface{}) {
	a.log(slog.LevelError, fmt.Sprintf(format, v...))
}

// ── CtxLogger interface ──

func (a *KitexAdapter) CtxTracef(ctx context.Context, format string, v ...interface{}) {
	a.logger.InfoContext(ctx, fmt.Sprintf(format, v...))
}
func (a *KitexAdapter) CtxDebugf(ctx context.Context, format string, v ...interface{}) {
	a.logger.InfoContext(ctx, fmt.Sprintf(format, v...))
}
func (a *KitexAdapter) CtxInfof(ctx context.Context, format string, v ...interface{}) {
	a.logger.InfoContext(ctx, fmt.Sprintf(format, v...))
}
func (a *KitexAdapter) CtxNoticef(ctx context.Context, format string, v ...interface{}) {
	a.logger.InfoContext(ctx, fmt.Sprintf(format, v...))
}
func (a *KitexAdapter) CtxWarnf(ctx context.Context, format string, v ...interface{}) {
	a.logger.WarnContext(ctx, fmt.Sprintf(format, v...))
}
func (a *KitexAdapter) CtxErrorf(ctx context.Context, format string, v ...interface{}) {
	a.logger.ErrorContext(ctx, fmt.Sprintf(format, v...))
}
func (a *KitexAdapter) CtxFatalf(ctx context.Context, format string, v ...interface{}) {
	a.logger.ErrorContext(ctx, fmt.Sprintf(format, v...))
}

// ── Control interface ──

func (a *KitexAdapter) SetLevel(level klog.Level) { a.level = level }
func (a *KitexAdapter) SetOutput(w io.Writer)     { a.writer = w }

func (a *KitexAdapter) log(level slog.Level, msg string) {
	a.logger.Logger.LogAttrs(context.Background(), level, msg)
}

type osStdout struct{}

func (osStdout) Write(p []byte) (int, error) { return len(p), nil }
