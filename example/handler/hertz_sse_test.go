package handler

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// 屏蔽测试用真实 Hertz server 的启动/关闭日志噪音，只保留 Error 及以上。
	hlog.SetLevel(hlog.LevelError)
}

// TestHandleSSEDemo 验证 GET /sse/demo 按 count 循环推送 message-i 事件。
//
// 说明：hertz v0.10.5 的 sse.Writer 依赖真实网络连接做 chunked flush，
// ut.PerformRequest 构造的内存态请求上下文不满足这一前提（会在首次
// WriteEvent 时 panic，见 go-framework/hertz/sse 包测试的同类问题），
// 因此这里改用真实 loopback Hertz server + 原始 TCP 读取来驱动测试。
func TestHandleSSEDemo(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())

	hostPort := "127.0.0.1:" + strconv.Itoa(port)
	h := server.New(server.WithHostPorts(hostPort), server.WithExitWaitTime(0))
	RegisterSSERoutes(h)

	go h.Spin()
	require.Eventually(t, func() bool {
		conn, dialErr := net.DialTimeout("tcp", hostPort, 20*time.Millisecond)
		if dialErr != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 2*time.Second, 10*time.Millisecond, "server did not start listening")

	conn, err := net.DialTimeout("tcp", hostPort, time.Second)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	_, err = conn.Write([]byte("GET /sse/demo?message=hi&count=2 HTTP/1.1\r\nHost: " + hostPort + "\r\nConnection: close\r\n\r\n"))
	require.NoError(t, err)

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, readErr := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if readErr != nil {
			break
		}
	}
	body := string(buf)

	assert.Contains(t, body, "event: message\n")
	assert.Contains(t, body, "data: hi-0\n")
	assert.Contains(t, body, "data: hi-1\n")
	assert.Contains(t, body, "Content-Type: text/event-stream; charset=utf-8")
}
