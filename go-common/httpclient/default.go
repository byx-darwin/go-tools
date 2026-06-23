package httpclient

import "time"

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
