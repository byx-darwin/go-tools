package httpclient

import (
	"math/rand"
	"time"

	"github.com/valyala/fasthttp"
)

// BodyFunc 是重试调用的函数签名。
//
// Deprecated: 新代码请使用 (*Client).Do，其内部重试逻辑已修复网络错误不重试的缺陷。
type BodyFunc func() (*fasthttp.Response, int, error)

// Retry 执行 fn，失败时最多重试 retries 次，每次间隔 sleep。
//
// Deprecated: 仅在 5xx 状态码时重试，网络层错误不会触发重试，属已知缺陷；
// 新代码请使用 (*Client).Do。
func Retry(retries int, sleep time.Duration, fn BodyFunc) (*fasthttp.Response, int, error) {
	if sleep == 0 {
		sleep = DefaultSleep
	}
	response, status, err := fn()
	if err != nil {
		return response, status, err
	}

	if status >= fasthttp.StatusInternalServerError {
		retries--
		if retries <= 0 {
			return nil, status, err
		}
		sleep += (time.Duration(rand.Int63n(int64(sleep)))) / 2
		time.Sleep(sleep)
		return Retry(retries, 2*sleep, fn)
	}
	return response, status, err
}
