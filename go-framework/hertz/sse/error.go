package sse

import (
	"encoding/json"
	"strconv"

	hertzsse "github.com/cloudwego/hertz/pkg/protocol/sse"
)

// sseErrorPayload SSE 错误事件负载，字段形状对齐 Response 三段式
// （code/msg/data），但 Code 语义不同：这里是 HTTP 状态码（如 500），
// 不是 Responder 正常 JSON 响应里的业务错误码（go-framework 10000-10499
// 段，见 CLAUDE.md D6）。SSE 场景没有走 Responder 的错误路由，客户端解析
// event:error 时不能按 Responder 响应的 code 语义处理。
type sseErrorPayload struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

// writeErrorEvent 写入一条 event:error，data 为 JSON 序列化的三段式结构。
// SSE 连接建立后响应头已提交为 text/event-stream，无法再切回 JSON/Protobuf
// 响应，因此错误一律走此事件格式，不复用 Responder.Error() 的内容协商逻辑。
func writeErrorEvent(w *hertzsse.Writer, code int, msg string) error {
	payload := sseErrorPayload{Code: code, Msg: msg}
	data, err := json.Marshal(payload)
	if err != nil {
		data = []byte(`{"code":` + strconv.Itoa(code) + `,"msg":"internal server error"}`)
	}
	return w.WriteEvent("", "error", data)
}
