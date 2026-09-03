package sse

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"

	"github.com/stretchr/testify/require"
)

func init() {
	// 屏蔽测试用真实 Hertz server 的启动/关闭日志噪音，只保留 Error 及以上。
	hlog.SetLevel(hlog.LevelError)
}

// startTestServer 启动一个监听本地随机端口的真实 Hertz server，注册
// GET /sse -> handler。
//
// Hertz v0.10.5 的 sse.Writer 依赖 app.RequestContext.GetWriter() 返回的
// network.Writer 落到真实连接上做 chunked flush；ut.PerformRequest /
// ut.CreateUtRequestContext 构造的 RequestContext 没有绑定网络连接，
// 调用 sse.NewWriter(...).WriteEvent(...) 会因为 network.Writer 为 nil
// 而 panic。因此这里改用真实 TCP server + net.Dial 读取原始响应字节的
// 方式驱动测试，而不是 ut 包提供的内存态请求上下文。
func startTestServer(t *testing.T, handler app.HandlerFunc) (addr string, stop func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())

	hostPort := "127.0.0.1:" + strconv.Itoa(port)
	h := server.New(
		server.WithHostPorts(hostPort),
		server.WithExitWaitTime(0),
	)
	h.GET("/sse", handler)

	ready := make(chan struct{})
	go func() {
		h.Spin()
	}()
	go func() {
		for i := 0; i < 200; i++ {
			if conn, dialErr := net.DialTimeout("tcp", hostPort, 20*time.Millisecond); dialErr == nil {
				_ = conn.Close()
				close(ready)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		close(ready)
	}()
	<-ready

	stopFn := func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = h.Shutdown(ctx)
	}
	return hostPort, stopFn
}

// rawHTTPGet 通过原始 TCP 连接向 /sse 发起 GET 请求，返回底层连接以便按需
// 持续读取 chunked SSE 响应体（不使用 net/http 客户端，避免其对 chunked
// body 的缓冲/重试语义掩盖了我们要观察的原始字节流时序）。
func rawHTTPGet(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	require.NoError(t, err)
	_, err = conn.Write([]byte("GET /sse HTTP/1.1\r\nHost: " + addr + "\r\nConnection: close\r\n\r\n"))
	require.NoError(t, err)
	return conn
}

// readAllWithTimeout 在 2 秒 deadline 内尽量读满 conn 直到 EOF 或超时。
func readAllWithTimeout(t *testing.T, conn net.Conn) string {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, err := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return string(buf)
}
