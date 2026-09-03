package sse

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	hertzsse "github.com/cloudwego/hertz/pkg/protocol/sse"

	"github.com/byx-darwin/go-tools/go-common/log"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
)

// ErrWriterClosed 表示 Writer 已关闭（客户端断连或业务主动 Close），
// 后续 WriteEvent 调用会立即返回此错误。
var ErrWriterClosed = errors.New("sse: writer closed")

// Writer 封装 Hertz 原生 SSE Writer，集成 Request ID、panic recovery、
// 心跳保活、断连检测，对齐 Responder 规范。
//
// 断连检测的真实机制：标准 Hertz handler 的 ctx 在客户端断连时不会自动
// cancel，实际生效的检测路径是心跳写失败（见 heartbeatLoop）。禁用心跳
// （WithHeartbeatInterval(0)）且 handler 纯阻塞等待数据源时，断连检测会
// 失效，可能导致 goroutine 泄漏，详见包文档「断连检测的真实机制」一节。
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
//
// 实现说明：真正的一次性关闭动作在 doClose 中完成；Close 在 doClose 之后
// 额外等待心跳 goroutine 退出（heartbeatDone），以保证 Close 返回时心跳
// goroutine 已完全停止。这个等待步骤只能由「心跳 goroutine 之外」的调用方
// 执行——heartbeatLoop 自身在 ctx 取消/写入失败时只调用 doClose（不等待），
// 否则会在同一个 goroutine 里等待自己的退出信号，造成死锁。
func (w *Writer) Close() error {
	err := w.doClose()
	if w.heartbeatDone != nil {
		<-w.heartbeatDone
	}
	return err
}

// doClose 执行一次性关闭：CAS 标记 closed、取消心跳 ctx、关闭底层 writer。
// 不等待心跳 goroutine 退出，heartbeatLoop 内部据此避免自等待死锁。
func (w *Writer) doClose() error {
	if !w.closed.CompareAndSwap(false, true) {
		return nil
	}
	if w.cancelHeartbeat != nil {
		w.cancelHeartbeat()
	}
	return w.w.Close()
}

// Run 包装业务事件循环：内部启动心跳 goroutine（heartbeatInterval<=0 时
// 仅监听 ctx.Done()，不发心跳），函数返回前自动 Close。handler 内 panic
// 会被捕获：调用 onRecover（若配置）→ 写入 event:error（500,
// "internal server error"）→ 记录结构化日志 → 不重新抛出。handler 的
// 返回值（含 nil）原样返回给调用方；panic 场景固定返回 nil，因为错误已经
// 通过 SSE 事件流交付给客户端。
//
// 断连检测警告：heartbeatInterval<=0 时，goroutine 只能靠 ctx.Done()
// 退出——而标准 Hertz handler 的 ctx 不会在客户端断连时自动 cancel。若
// handler 同时是纯阻塞等待（无自带超时/取消），断连后 goroutine 会无限
// 阻塞（泄漏）。禁用心跳前务必确认 handler 有其他方式感知断连或超时退出。
func (w *Writer) Run(handler func(w *Writer) error) (err error) {
	ctx, cancel := context.WithCancel(w.runCtx())
	w.cancelHeartbeat = cancel
	w.heartbeatDone = make(chan struct{})
	go w.heartbeatLoop(ctx)

	defer func() {
		if rec := recover(); rec != nil {
			if w.cfg.onRecover != nil {
				w.cfg.onRecover(rec)
			}
			log.L().WithCategory(log.CategoryPanic).ErrorContext(ctx, "sse handler panic recovered",
				fmt.Errorf("%v", rec),
				"request_id", w.requestID,
				"panic", fmt.Sprintf("%v", rec),
			)
			_ = writeErrorEvent(w.w, 500, "internal server error")
			err = nil
		}
		_ = w.Close()
	}()

	return handler(w)
}

// heartbeatLoop 后台心跳 goroutine，同时是当前唯一可靠的断连检测路径：
// WriteKeepAlive() 在底层 socket 已断开时返回 error，据此触发 doClose()。
// 标准 Hertz handler 的 ctx 不会在客户端断连时自动 cancel，因此
// heartbeatInterval<=0（跳过心跳，仅监听 ctx.Done()）场景下，若无外部
// cancel 来源，断连不会被感知——见 Run 与包文档的警告。
func (w *Writer) heartbeatLoop(ctx context.Context) {
	defer close(w.heartbeatDone)

	if w.cfg.heartbeatInterval <= 0 {
		<-ctx.Done()
		_ = w.doClose()
		return
	}

	ticker := time.NewTicker(w.cfg.heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := w.w.WriteKeepAlive(); err != nil {
				_ = w.doClose()
				return
			}
		case <-ctx.Done():
			_ = w.doClose()
			return
		}
	}
}

// runCtx 返回 Run 使用的断连检测 context；NewWriter 保存的原始 ctx 为空时
// （不应发生，防御性兜底）退化为 context.Background()。
func (w *Writer) runCtx() context.Context {
	if w.parentCtx != nil {
		return w.parentCtx
	}
	return context.Background()
}
