// Package rpc 提供 Kitex RPC 服务端和客户端的实现。
package rpc

import (
	"context"
	"fmt"

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
