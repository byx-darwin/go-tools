package sse

import (
	"encoding/json"
	"strconv"

	hertzsse "github.com/cloudwego/hertz/pkg/protocol/sse"
)

// sseErrorPayload SSE 错误事件负载，对齐 Response 三段式（code/msg/data）。
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
