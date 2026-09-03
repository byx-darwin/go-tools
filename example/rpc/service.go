// Package rpc 提供 Kitex RPC 服务端和客户端的实现。
package rpc

import (
	"context"
	"fmt"
	"time"

	demo "github.com/byx-darwin/go-tools/example/kitex_generated/demo"
)

// DemoServiceImpl 实现 demo.DemoService 接口。
type DemoServiceImpl struct{}

// Echo 回显消息，如果 message 为 "error" 则返回错误。
func (s *DemoServiceImpl) Echo(_ context.Context, req *demo.EchoRequest) (*demo.EchoResponse, error) {
	if req.GetMessage() == "error" {
		return nil, fmt.Errorf("demo echo error: received 'error' message")
	}

	return &demo.EchoResponse{
		Message: req.GetMessage(),
		Service: "go-tools-example",
	}, nil
}

// Health 返回服务健康状态。
func (s *DemoServiceImpl) Health(_ context.Context, _ *demo.HealthRequest) (*demo.HealthResponse, error) {
	return &demo.HealthResponse{
		Healthy: true,
		Version: "v1.0.0",
	}, nil
}

// StreamEcho 按 req.Message 中的字符逐个发送流式响应，演示 server-streaming。
// 每帧之间 sleep 10ms，便于客户端观测到多帧到达。
func (s *DemoServiceImpl) StreamEcho(req *demo.EchoRequest, stream demo.DemoService_StreamEchoServer) error {
	msg := req.GetMessage()
	if msg == "" {
		msg = "hello"
	}
	for _, r := range msg {
		if err := stream.Send(&demo.EchoResponse{
			Message: string(r),
			Service: "go-tools-example",
		}); err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}
