package log

import (
	"context"
	"log/slog"
)

// categoryHandler 在日志中注入 category 字段。
type categoryHandler struct {
	next     slog.Handler
	category string
}

// NewCategoryHandler 创建 category handler。
func NewCategoryHandler(next slog.Handler, category string) slog.Handler {
	return &categoryHandler{next: next, category: category}
}

func (h *categoryHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *categoryHandler) Handle(ctx context.Context, r slog.Record) error {
	r.AddAttrs(slog.String("category", h.category))
	return h.next.Handle(ctx, r)
}

func (h *categoryHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &categoryHandler{next: h.next.WithAttrs(attrs), category: h.category}
}

func (h *categoryHandler) WithGroup(name string) slog.Handler {
	return &categoryHandler{next: h.next.WithGroup(name), category: h.category}
}

// releaseHandler 在日志中注入发布信息。
type releaseHandler struct {
	next    slog.Handler
	release ReleaseInfo
}

// NewReleaseHandler 创建 release handler。
func NewReleaseHandler(next slog.Handler, release ReleaseInfo) slog.Handler {
	return &releaseHandler{next: next, release: release}
}

func (h *releaseHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *releaseHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.release.ServiceName != "" {
		r.AddAttrs(slog.String("service.name", h.release.ServiceName))
	}
	if h.release.Version != "" {
		r.AddAttrs(slog.String("service.version", h.release.Version))
	}
	if h.release.Environment != "" {
		r.AddAttrs(slog.String("environment", h.release.Environment))
	}
	for k, v := range h.release.Extra {
		r.AddAttrs(slog.String(k, v))
	}
	return h.next.Handle(ctx, r)
}

func (h *releaseHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &releaseHandler{next: h.next.WithAttrs(attrs), release: h.release}
}

func (h *releaseHandler) WithGroup(name string) slog.Handler {
	return &releaseHandler{next: h.next.WithGroup(name), release: h.release}
}

// contextHandler 从 context 中提取并注入字段。
type contextHandler struct {
	next slog.Handler
}

// NewContextHandler 创建 context handler。
func NewContextHandler(next slog.Handler) slog.Handler {
	return &contextHandler{next: next}
}

func (h *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if requestID := RequestIDFromContext(ctx); requestID != "" {
		r.AddAttrs(slog.String("request_id", requestID))
	}
	return h.next.Handle(ctx, r)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{next: h.next.WithAttrs(attrs)}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{next: h.next.WithGroup(name)}
}

// maskHandler 对日志属性进行脱敏处理。
type maskHandler struct {
	next   slog.Handler
	masker *Masker
}

// NewMaskHandler 创建 mask handler。
func NewMaskHandler(next slog.Handler, masker *Masker) slog.Handler {
	return &maskHandler{next: next, masker: masker}
}

func (h *maskHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *maskHandler) Handle(ctx context.Context, r slog.Record) error {
	var attrs []slog.Attr
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})
	masked := h.masker.Mask(attrs)
	r = slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	for _, attr := range masked {
		r.AddAttrs(attr)
	}
	return h.next.Handle(ctx, r)
}

func (h *maskHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &maskHandler{next: h.next.WithAttrs(attrs), masker: h.masker}
}

func (h *maskHandler) WithGroup(name string) slog.Handler {
	return &maskHandler{next: h.next.WithGroup(name), masker: h.masker}
}

// multiHandler 将日志输出到多个 handler（fan-out）。
type multiHandler struct {
	handlers []slog.Handler
}

// NewMultiHandler 创建多输出 handler。
func NewMultiHandler(handlers ...slog.Handler) slog.Handler {
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for i, handler := range h.handlers {
		rec := r
		if i > 0 {
			// 为后续 handler 克隆 record，避免 handler 修改 attrs 导致竞态
			rec = slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
			r.Attrs(func(a slog.Attr) bool {
				rec.AddAttrs(a)
				return true
			})
		}
		if err := handler.Handle(ctx, rec); err != nil {
			return err
		}
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}

// domainHandler 在日志中注入 domain 和 log_type 字段。
type domainHandler struct {
	next    slog.Handler
	domain  string
	logType string
}

// NewDomainHandler 创建 domain handler。
func NewDomainHandler(next slog.Handler, domain, logType string) slog.Handler {
	return &domainHandler{next: next, domain: domain, logType: logType}
}

func (h *domainHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *domainHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.domain != "" {
		r.AddAttrs(slog.String("domain", h.domain))
	}
	if h.logType != "" {
		r.AddAttrs(slog.String("log_type", h.logType))
	}
	return h.next.Handle(ctx, r)
}

func (h *domainHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &domainHandler{next: h.next.WithAttrs(attrs), domain: h.domain, logType: h.logType}
}

func (h *domainHandler) WithGroup(name string) slog.Handler {
	return &domainHandler{next: h.next.WithGroup(name), domain: h.domain, logType: h.logType}
}
