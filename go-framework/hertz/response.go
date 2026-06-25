package hertz

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

// ── 响应体 ──

// Response 统一响应体。
// 支持 JSON 和 Protobuf 双格式序列化。
type Response struct {
	Code int    `json:"code" protobuf:"varint,1,opt,name=code"`
	Msg  string `json:"msg"  protobuf:"bytes,2,opt,name=msg"`
	Data any    `json:"data,omitempty" protobuf:"bytes,3,opt,name=data"`
}

// ── I18n 接口 ──

// Translator 国际化翻译器接口。
// 项目方实现此接口接入自己的 i18n 系统。
type Translator interface {
	// Translate 翻译消息 key 为指定语言文本。
	Translate(ctx context.Context, lang, key string) string
}

// ── 错误路由接口 ──

// ErrorRoute 错误路由结果。
type ErrorRoute struct {
	HTTPCode int    // HTTP 状态码（如 200, 400, 500）
	BizCode  int    // 业务码（响应体中的 code 字段）
	Override string // 覆盖消息（非空时替代 publicMsg）
}

// ErrorRouter RPC 错误路由器接口。
// 将 RPC 错误映射为 HTTP 响应参数。
type ErrorRouter interface {
	// Route 分析错误，返回路由结果。
	// 返回 ok=false 表示不识别此错误，走默认路由。
	Route(ctx context.Context, err error) (ErrorRoute, bool)
}

// ── 默认错误路由器 ──

// RPCErrorRouter 基于 go-common/error 的默认路由器。
// 从 oops 错误中提取错误码（10001, 10002 等），
// 使用 goerror.HTTPStatus() 映射 HTTP 状态码。
type RPCErrorRouter struct{}

// Route 分析 oops 错误，提取错误码和 HTTP 状态码。
// 非 oops 错误返回 ok=false。
func (r *RPCErrorRouter) Route(_ context.Context, err error) (ErrorRoute, bool) {
	if err == nil {
		return ErrorRoute{}, false
	}
	code, public := goerror.Extract(err)
	if code == 0 {
		return ErrorRoute{}, false
	}
	return ErrorRoute{
		HTTPCode: goerror.HTTPStatus(err),
		BizCode:  code,
		Override: public,
	}, true
}

// ── 默认常量 ──

const (
	defaultReqIDHeader = "X-Request-ID"
	defaultLangHeader  = "Accept-Language"
)

// ── Responder ──

// Responder 统一响应处理器。
// 持有配置，提供 Success/Error/Reply 方法。
// 通过 Middleware 注入请求上下文。
type Responder struct {
	debug       bool
	translator  Translator
	errorRouter ErrorRouter
	reqIDHeader string
	reqIDGen    func() string
	langHeader  string
	successCode int
	failCode    int
}

// NewResponder 创建 Responder 实例。
// 默认配置：
//   - debug: false
//   - reqIDHeader: "X-Request-ID"
//   - langHeader: "Accept-Language"
//   - successCode: 200
//   - failCode: 500
func NewResponder(opts ...Option) *Responder {
	r := &Responder{
		reqIDHeader: defaultReqIDHeader,
		langHeader:  defaultLangHeader,
		successCode: http.StatusOK,
		failCode:    http.StatusInternalServerError,
		reqIDGen:    uuid.NewString,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// ── Functional Options ──

// Option 定义 Responder 配置选项。
type Option func(*Responder)

// WithDebug 启用 Debug 模式。
// Debug 模式下错误响应包含内部错误详情，生产环境应关闭。
func WithDebug(debug bool) Option {
	return func(r *Responder) { r.debug = debug }
}

// WithTranslator 设置 i18n 翻译器。
func WithTranslator(t Translator) Option {
	return func(r *Responder) { r.translator = t }
}

// WithErrorRouter 设置错误路由器。
// 传入 nil 清除路由器。
func WithErrorRouter(e ErrorRouter) Option {
	return func(r *Responder) { r.errorRouter = e }
}

// WithRequestIDHeader 设置 Request ID 响应头名称。
// 默认 "X-Request-ID"。空字符串禁用 Request ID 功能。
func WithRequestIDHeader(name string) Option {
	return func(r *Responder) { r.reqIDHeader = name }
}

// WithRequestIDGenerator 设置 Request ID 生成函数。
// 默认使用 UUID v4。
func WithRequestIDGenerator(fn func() string) Option {
	return func(r *Responder) {
		if fn != nil {
			r.reqIDGen = fn
		}
	}
}

// WithLangHeader 设置语言检测请求头名称。
// 默认 "Accept-Language"。空字符串禁用 i18n 语言检测。
func WithLangHeader(name string) Option {
	return func(r *Responder) { r.langHeader = name }
}

// WithDefaultBizCode 设置默认成功/失败业务码。
// 默认 successCode=200, failCode=500。
func WithDefaultBizCode(successCode, failCode int) Option {
	return func(r *Responder) {
		if successCode != 0 {
			r.successCode = successCode
		}
		if failCode != 0 {
			r.failCode = failCode
		}
	}
}

// ── Request ID ──

// extractRequestID 提取请求 ID。
// 优先级：OTel trace-id → X-Request-ID header → UUID 生成。
func (r *Responder) extractRequestID(ctx context.Context, rc *app.RequestContext) string {
	// 1. OTel trace-id（hex 格式）
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	// 2. X-Request-ID 请求头
	if r.reqIDHeader != "" {
		if id := string(rc.Request.Header.Peek(r.reqIDHeader)); id != "" {
			return id
		}
	}
	// 3. 生成 UUID
	if r.reqIDGen != nil {
		return r.reqIDGen()
	}
	return ""
}

// ── 内容协商 ──

// negotiateContentType 根据 Accept 头决定响应格式。
func negotiateContentType(ctx *app.RequestContext) string {
	accept := strings.ToLower(string(ctx.Request.Header.Peek(consts.HeaderAccept)))
	if strings.Contains(accept, consts.MIMEPROTOBUF) {
		return consts.MIMEPROTOBUF
	}
	return consts.MIMEApplicationJSONUTF8
}

// writeResponse 根据内容协商写入响应。
func (r *Responder) writeResponse(ctx *app.RequestContext, httpCode int, obj any) {
	switch negotiateContentType(ctx) {
	case consts.MIMEPROTOBUF:
		ctx.ProtoBuf(httpCode, obj)
	default:
		ctx.JSON(httpCode, obj)
	}
}

// ── 响应方法 ──

// reply 内部写响应方法。
func (r *Responder) reply(ctx *app.RequestContext, httpCode, bizCode int, data any, msg string) {
	resp := Response{Code: bizCode, Msg: msg, Data: data}
	r.writeResponse(ctx, httpCode, resp)
}

// Success 成功响应。
// HTTP 200，业务码为 successCode（默认 200），msg 为 "ok"。
// data 为 nil 时响应体不含 data 字段。
func (r *Responder) Success(ctx *app.RequestContext, data any) {
	r.reply(ctx, http.StatusOK, r.successCode, data, "ok")
}

// SuccessWithMsg 带自定义消息的成功响应。
// msg 会经过 Translator 翻译（若已配置）。
// c 为请求的 context.Context，用于 i18n 翻译和链路追踪。
func (r *Responder) SuccessWithMsg(c context.Context, ctx *app.RequestContext, data any, msg string) {
	r.reply(ctx, http.StatusOK, r.successCode, data, r.translate(c, ctx, msg))
}

// Error 错误响应。
// 根据错误类型自动路由：
//  1. 若配置了 ErrorRouter 且识别错误 → 使用路由结果
//  2. 否则 → HTTP 500 + failCode
//
// publicMsg 为用户可见消息（经 Translator 翻译）。
// Debug 模式下，err.Error() 附加到 msg 末尾。
// c 为请求的 context.Context，用于 ErrorRouter.Route 和 i18n 翻译。
func (r *Responder) Error(c context.Context, ctx *app.RequestContext, err error, publicMsg string) {
	httpCode, bizCode, finalMsg := r.routeError(c, ctx, err, publicMsg)
	r.reply(ctx, httpCode, bizCode, nil, finalMsg)
}

// ErrorWithCode 指定业务码的错误响应。
// 跳过 ErrorRouter，直接使用指定 bizCode。
// c 为请求的 context.Context，用于 i18n 翻译。
func (r *Responder) ErrorWithCode(c context.Context, ctx *app.RequestContext, httpCode, bizCode int, msg string) {
	r.reply(ctx, httpCode, bizCode, nil, r.translate(c, ctx, msg))
}

// Reply 原始响应写入。
// 直接写入指定 HTTP 状态码和对象，用于自定义响应结构。
// 支持内容协商（JSON/Protobuf）。
func (r *Responder) Reply(ctx *app.RequestContext, httpCode int, obj any) {
	r.writeResponse(ctx, httpCode, obj)
}

// ── 错误路由 ──

// routeError 解析错误，返回 HTTP 状态码、业务码和最终消息。
// c 为请求的 context.Context，用于 ErrorRouter.Route。
func (r *Responder) routeError(c context.Context, ctx *app.RequestContext, err error, publicMsg string) (httpCode, bizCode int, finalMsg string) {
	// 1. 尝试自定义 ErrorRouter
	if r.errorRouter != nil {
		if route, ok := r.errorRouter.Route(c, err); ok {
			msg := publicMsg
			if route.Override != "" {
				msg = route.Override
			}
			return route.HTTPCode, route.BizCode, r.applyDebugFilter(msg, err)
		}
	}
	// 2. 兜底
	return http.StatusInternalServerError, r.failCode, r.applyDebugFilter(r.translate(c, ctx, publicMsg), err)
}

// ── Debug 模式 ──

// applyDebugFilter Debug 模式下附加内部错误详情。
// 安全警告：Debug 模式绝不能在正式生产环境启用。
// err.Error() 可能包含敏感信息（SQL 语句、堆栈路径等）。
func (r *Responder) applyDebugFilter(msg string, err error) string {
	if !r.debug || err == nil {
		return msg
	}
	return fmt.Sprintf("%s | internal: %s", msg, err.Error())
}

// ── I18n ──

// extractLang 从请求头提取语言偏好。
func (r *Responder) extractLang(ctx *app.RequestContext) string {
	if r.langHeader == "" {
		return ""
	}
	lang := string(ctx.Request.Header.Peek(r.langHeader))
	if lang == "" {
		return "zh"
	}
	// 解析 "zh-CN,zh;q=0.9" → "zh"
	if idx := strings.IndexAny(lang, ",;"); idx > 0 {
		lang = lang[:idx]
	}
	if idx := strings.IndexByte(lang, '-'); idx > 0 {
		lang = lang[:idx]
	}
	return lang
}

// translate 翻译消息（若已配置 Translator）。
// c 为请求的 context.Context，用于 Translator.Translate。
func (r *Responder) translate(c context.Context, ctx *app.RequestContext, msg string) string {
	if r.translator == nil || msg == "" {
		return msg
	}
	lang := r.extractLang(ctx)
	return r.translator.Translate(c, lang, msg)
}

// ── Context Keys ──

type ctxKey string

const (
	ctxKeyRequestID ctxKey = "responder:request_id"
	ctxKeyLang      ctxKey = "responder:lang"
	ctxKeyResponder ctxKey = "responder:instance"
)

// ── Middleware ──

// Middleware 返回 Hertz 中间件处理函数。
// 中间件职责：
//  1. 提取/生成 Request ID → 设置响应头 + 注入 ctx
//  2. 提取语言偏好 → 注入 ctx
//  3. 注入 Responder 实例 → 注入 ctx
func (r *Responder) Middleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// 1. 提取 Request ID
		reqID := r.extractRequestID(ctx, c)
		if reqID != "" && r.reqIDHeader != "" {
			c.Response.Header.Set(r.reqIDHeader, reqID)
		}
		c.Set(string(ctxKeyRequestID), reqID)

		// 2. 提取语言偏好
		lang := r.extractLang(c)
		c.Set(string(ctxKeyLang), lang)

		// 3. 注入 Responder
		c.Set(string(ctxKeyResponder), r)

		c.Next(ctx)
	}
}

// ── Context 辅助函数 ──

var defaultResponder = NewResponder()

// RespondFrom 从请求上下文获取 Responder。
// 若未通过 Middleware 注入，返回默认 Responder。
func RespondFrom(ctx *app.RequestContext) *Responder {
	if v, ok := ctx.Value(string(ctxKeyResponder)).(*Responder); ok {
		return v
	}
	return defaultResponder
}

// RequestIDFrom 从请求上下文获取当前 Request ID。
// 若未通过 Middleware 提取，返回空字符串。
func RequestIDFrom(ctx *app.RequestContext) string {
	if v, ok := ctx.Value(string(ctxKeyRequestID)).(string); ok {
		return v
	}
	return ""
}

// ── 包级便捷函数（使用默认 Responder）──

// Success 成功响应（使用默认 Responder）。
func Success(ctx *app.RequestContext, data any) {
	defaultResponder.Success(ctx, data)
}

// Error 错误响应（使用默认 Responder）。
func Error(c context.Context, ctx *app.RequestContext, err error, publicMsg string) {
	defaultResponder.Error(c, ctx, err, publicMsg)
}

// ErrorWithCode 指定业务码的错误响应（使用默认 Responder）。
func ErrorWithCode(c context.Context, ctx *app.RequestContext, httpCode, bizCode int, msg string) {
	defaultResponder.ErrorWithCode(c, ctx, httpCode, bizCode, msg)
}

// Reply 原始响应写入（使用默认 Responder）。
func Reply(ctx *app.RequestContext, httpCode int, obj any) {
	defaultResponder.Reply(ctx, httpCode, obj)
}
