package hertz

import (
	"context"
	"net/http"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/google/uuid"
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
