// Package adapters 提供 slog.Logger 到框架日志接口的适配器。
package adapters

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"gitee.com/byx_darwin/go-tools/go-common/log"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// HertzAdapter 将 go-common/log.Logger 适配为 hertz hlog.FullLogger。
//
// 用法:
//
//	l := log.New(log.WithLevel("info"))
//	hlog.SetLogger(adapters.NewHertzAdapter(l))
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

func (a *HertzAdapter) Trace(v ...interface{})  { a.log(slog.LevelDebug, fmt.Sprint(v...)) }
func (a *HertzAdapter) Debug(v ...interface{})  { a.log(slog.LevelDebug, fmt.Sprint(v...)) }
func (a *HertzAdapter) Info(v ...interface{})   { a.log(slog.LevelInfo, fmt.Sprint(v...)) }
func (a *HertzAdapter) Notice(v ...interface{}) { a.log(slog.LevelInfo, fmt.Sprint(v...)) }
func (a *HertzAdapter) Warn(v ...interface{})   { a.log(slog.LevelWarn, fmt.Sprint(v...)) }
func (a *HertzAdapter) Error(v ...interface{})  { a.log(slog.LevelError, fmt.Sprint(v...)) }
func (a *HertzAdapter) Fatal(v ...interface{})  { a.log(slog.LevelError, fmt.Sprint(v...)) }

// ── FormatLogger interface ──

func (a *HertzAdapter) Tracef(format string, v ...interface{}) {
	a.log(slog.LevelDebug, fmt.Sprintf(format, v...))
}
func (a *HertzAdapter) Debugf(format string, v ...interface{}) {
	a.log(slog.LevelDebug, fmt.Sprintf(format, v...))
}
func (a *HertzAdapter) Infof(format string, v ...interface{}) {
	a.log(slog.LevelInfo, fmt.Sprintf(format, v...))
}
func (a *HertzAdapter) Noticef(format string, v ...interface{}) {
	a.log(slog.LevelInfo, fmt.Sprintf(format, v...))
}
func (a *HertzAdapter) Warnf(format string, v ...interface{}) {
	a.log(slog.LevelWarn, fmt.Sprintf(format, v...))
}
func (a *HertzAdapter) Errorf(format string, v ...interface{}) {
	a.log(slog.LevelError, fmt.Sprintf(format, v...))
}
func (a *HertzAdapter) Fatalf(format string, v ...interface{}) {
	a.log(slog.LevelError, fmt.Sprintf(format, v...))
}

// ── CtxLogger interface ──

func (a *HertzAdapter) CtxTracef(ctx context.Context, format string, v ...interface{}) {
	a.logger.InfoContext(ctx, fmt.Sprintf(format, v...))
}
func (a *HertzAdapter) CtxDebugf(ctx context.Context, format string, v ...interface{}) {
	a.logger.InfoContext(ctx, fmt.Sprintf(format, v...))
}
func (a *HertzAdapter) CtxInfof(ctx context.Context, format string, v ...interface{}) {
	a.logger.InfoContext(ctx, fmt.Sprintf(format, v...))
}
func (a *HertzAdapter) CtxNoticef(ctx context.Context, format string, v ...interface{}) {
	a.logger.InfoContext(ctx, fmt.Sprintf(format, v...))
}
func (a *HertzAdapter) CtxWarnf(ctx context.Context, format string, v ...interface{}) {
	a.logger.WarnContext(ctx, fmt.Sprintf(format, v...))
}
func (a *HertzAdapter) CtxErrorf(ctx context.Context, format string, v ...interface{}) {
	a.logger.ErrorContext(ctx, fmt.Sprintf(format, v...))
}
func (a *HertzAdapter) CtxFatalf(ctx context.Context, format string, v ...interface{}) {
	a.logger.ErrorContext(ctx, fmt.Sprintf(format, v...))
}

// ── Control interface ──

func (a *HertzAdapter) SetLevel(level hlog.Level) { a.level = level }
func (a *HertzAdapter) SetOutput(w io.Writer)     { a.writer = w }

func (a *HertzAdapter) log(level slog.Level, msg string) {
	a.logger.Logger.LogAttrs(context.Background(), level, msg)
}
