package httpclient

import "time"

// 默认常量和 HTTP 方法。
const (
	DefaultSleep    = 500 * time.Millisecond
	MethodGet       = "GET"
	MethodHead      = "HEAD"
	MethodPost      = "POST"
	MethodPut       = "PUT"
	MethodPatch     = "PATCH"
	MethodDelete    = "DELETE"
	MethodConnect   = "CONNECT"
	MethodOptions   = "OPTIONS"
	MethodTrace     = "TRACE"
	FasthttpVersion = "v1.61.0"
)
