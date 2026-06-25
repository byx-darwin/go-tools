package hertz

import (
	"context"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
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
