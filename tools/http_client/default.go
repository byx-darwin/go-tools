package http_client

import "time"

const (
	// 默认休眠时间为 500 毫秒
	DefaultSleep = 500 * time.Millisecond

	MethodGet       = "GET"     // RFC 7231, 4.3.1
	MethodHead      = "HEAD"    // RFC 7231, 4.3.2
	MethodPost      = "POST"    // RFC 7231, 4.3.3
	MethodPut       = "PUT"     // RFC 7231, 4.3.4
	MethodPatch     = "PATCH"   // RFC 5789
	MethodDelete    = "DELETE"  // RFC 7231, 4.3.5
	MethodConnect   = "CONNECT" // RFC 7231, 4.3.6
	MethodOptions   = "OPTIONS" // RFC 7231, 4.3.7
	MethodTrace     = "TRACE"   // RFC 7231, 4.3.8
	FasthttpVersion = "v1.61.0"
)
