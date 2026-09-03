package rpc

import (
	"context"
	"errors"
	"fmt"
	"io"

	demo "github.com/byx-darwin/go-tools/example/kitex_generated/demo"
	"github.com/byx-darwin/go-tools/example/kitex_generated/demo/demoservice"
	kitexobs "github.com/byx-darwin/go-tools/go-framework/kitex/observability"

	"github.com/cloudwego/kitex/client"
)

// NewDemoClient 创建 Kitex DemoService 客户端。
//
// 连接到 rpcAddr（如 "localhost:8888"），如果 obsProvider 不为 nil 则注入 OTel 客户端 Suite。
func NewDemoClient(rpcAddr string, obsProvider *kitexobs.Provider) (demoservice.Client, error) {
	var opts []client.Option
	opts = append(opts, client.WithHostPorts(rpcAddr))

	// 注入 OTel 可观测性。
	if obsProvider != nil && obsProvider.Enabled() {
		suite := obsProvider.ClientSuite()
		opts = append(opts, suite.Options()...)
	}

	c, err := demoservice.NewClient("go-tools-example", opts...)
	if err != nil {
		return nil, fmt.Errorf("create kitex client: %w", err)
	}
	return c, nil
}

// CallStreamEcho 演示流式客户端调用：逐帧接收 StreamEcho 响应直至 io.EOF。
func CallStreamEcho(ctx context.Context, c demoservice.Client, message string) ([]string, error) {
	stream, err := c.StreamEcho(ctx, &demo.EchoRequest{Message: message})
	if err != nil {
		return nil, fmt.Errorf("open StreamEcho: %w", err)
	}
	var frames []string
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return frames, fmt.Errorf("recv StreamEcho frame: %w", err)
		}
		frames = append(frames, resp.GetMessage())
	}
	return frames, nil
}
