package http_client

import (
	"math/rand"
	"time"
)
import "github.com/valyala/fasthttp"

type BodyFunc func() (*fasthttp.Response, int, error)

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
