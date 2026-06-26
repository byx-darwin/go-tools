package rpc

import (
	"fmt"

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
