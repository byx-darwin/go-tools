package httpclient

import (
	"github.com/valyala/fasthttp"
	"strconv"
	"time"
)

// GetM3u8TsSize 函数用于获取 m3u8 文件的 ts 分片大小
func GetM3u8TsSize(httpUrl string, retries int, sleep, timeout time.Duration) (uint64, error) {
	headers := make(map[string]string)
	headers["Range"] = "bytes=0-0"
	rsp, err := SendWithRetry(httpUrl, MethodGet, nil, headers, sleep, timeout, retries)
	if err != nil {
		return 0, err
	}
	defer fasthttp.ReleaseResponse(rsp)
	contentLength := string(rsp.Header.Peek("Content-Length"))
	size := 0
	if contentLength != "" {
		size, _ = strconv.Atoi(contentLength)
	}
	return uint64(size), nil
}

// DownloadM3u8TsData 下载 m3u8 视频文件中的指定 ts 数据段
func DownloadM3u8TsData(httpUrl string, start, end, retries int, sleep, timeout time.Duration) ([]byte, error) {
	rangeValue := "bytes=" + strconv.Itoa(start) + "-" + strconv.Itoa(end)
	headers := make(map[string]string)
	headers["Range"] = rangeValue
	rsp, err := SendWithRetry(httpUrl, MethodGet, nil, headers, sleep, timeout, retries)
	if err != nil {
		return nil, err
	}
	defer fasthttp.ReleaseResponse(rsp)
	return rsp.Body(), nil
}
