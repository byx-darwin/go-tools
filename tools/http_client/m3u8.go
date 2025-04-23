package http_client

import (
	"github.com/valyala/fasthttp"
	"strconv"
	"time"
)

// GetM3u8TsSize 函数用于获取 m3u8 文件的 ts 分片大小。
// 它通过发送带有 Range 头部的 GET 请求到指定的 m3u8 URL 来获取文件的大小。
// 参数:
//
//	httpUrl: m3u8 文件的 URL。
//	retries: 重试次数，用于在网络失败时重试请求。
//	sleep: 重试之间的等待时间。
//	timeout: 请求超时时间。
//
// 返回值:
//   - uint64: m3u8 文件的大小。
//   - error: 如果请求失败，返回相应的错误信息。
func GetM3u8TsSize(httpUrl string, retries int, sleep, timeout time.Duration) (uint64, error) {
	// 创建一个包含Range头的请求头
	headers := make(map[string]string)
	// 请求从第0个字节到第0个字节，即请求整个文件
	headers["Range"] = "bytes=0-0"
	// 发送带有重试机制的HTTP GET请求
	rsp, err := SendWithRetry(httpUrl, MethodGet, nil, headers, sleep, timeout, retries)
	if err != nil {
		// 如果发送请求失败，返回错误
		return 0, err
	}
	// 确保响应体被正确释放
	defer fasthttp.ReleaseResponse(rsp)
	// 从响应头中获取Content-Length
	contentLength := string(rsp.Header.Peek("Content-Length"))
	size := 0
	if contentLength != "" {
		size, _ = strconv.Atoi(contentLength)
	}
	// 返回文件大小
	return uint64(size), nil
}

// DownloadM3u8TsData 函数用于下载 m3u8 视频文件中的指定 ts 数据段。
// 参数:
// httpUrl 是 m3u8 文件的 URL。
// start 和 end 是要下载的 ts 数据段的起始和结束字节位置。
// retries 是下载失败时的重试次数。
// sleep 是每次重试之间的等待时间。
// timeout 是 HTTP 请求的超时时间。
// 返回值:
//   - []byte: m3u8 文件的数据。
//   - error: 如果请求失败，返回相应的错误信息。
func DownloadM3u8TsData(httpUrl string, start, end, retries int, sleep, timeout time.Duration) ([]byte, error) {
	// 构建 Range 请求头，用于指定下载的字节范围
	rangeValue := "bytes=" + strconv.Itoa(start) + "-" + strconv.Itoa(end)
	// 创建包含 Range 请求头的 headers
	headers := make(map[string]string)
	headers["Range"] = rangeValue
	// 发送带有重试机制的 HTTP GET 请求
	rsp, err := SendWithRetry(httpUrl, MethodGet, nil, headers, sleep, timeout, retries)
	if err != nil {
		return nil, err
	}
	// 释放响应对象
	defer fasthttp.ReleaseResponse(rsp)
	// 返回响应体
	return rsp.Body(), nil
}
