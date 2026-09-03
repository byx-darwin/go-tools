package sse

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/cloudwego/hertz/pkg/app"
	hertzsse "github.com/cloudwego/hertz/pkg/protocol/sse"

	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
)

// ErrWriterClosed 表示 Writer 已关闭（客户端断连或业务主动 Close），
// 后续 WriteEvent 调用会立即返回此错误。
var ErrWriterClosed = errors.New("sse: writer closed")

// Writer 封装 Hertz 原生 SSE Writer，集成 Request ID、panic recovery、
// 心跳保活、断连检测，对齐 Responder 规范。
type Writer struct {
	w         *hertzsse.Writer
	cfg       config
	requestID string
	closed    atomic.Bool
	parentCtx context.Context // retained for Run 的断连检测 goroutine

	cancelHeartbeat context.CancelFunc
	heartbeatDone   chan struct{}
}

// NewWriter 创建 SSE Writer，立即写入 SSE 响应头
// （Content-Type: text/event-stream; charset=utf-8，Cache-Control: no-cache）。
//
// 默认配置：
//   - heartbeatInterval: 15 * time.Second
//   - onRecover: nil（仅记录结构化日志）
func NewWriter(c context.Context, rc *app.RequestContext, opts ...Option) *Writer {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Writer{
		w:         hertzsse.NewWriter(rc),
		cfg:       cfg,
		requestID: hertzresp.RequestIDFrom(rc),
		parentCtx: c,
	}
}

// WriteEvent 写入一条 SSE 事件（透传 hertz sse.Writer.WriteEvent）。
// Writer 已关闭时立即返回 ErrWriterClosed；断连或写入失败时返回底层错误，
// 调用方应据此退出事件循环。
func (w *Writer) WriteEvent(id, eventType string, data []byte) error {
	if w.closed.Load() {
		return ErrWriterClosed
	}
	return w.w.WriteEvent(id, eventType, data)
}

// Close 关闭连接，停止心跳 goroutine（若已启动）。幂等：多次调用安全。
func (w *Writer) Close() error {
	if !w.closed.CompareAndSwap(false, true) {
		return nil
	}
	if w.cancelHeartbeat != nil {
		w.cancelHeartbeat()
		<-w.heartbeatDone
	}
	return w.w.Close()
}
