package rpc

import (
	"context"
	"net"
	"strconv"

	"github.com/byx-darwin/go-tools/example/kitex_generated/demo/demoservice"
	"github.com/byx-darwin/go-tools/go-common/log"
	kitexobs "github.com/byx-darwin/go-tools/go-framework/kitex/observability"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/server"
)

// StartServer 启动 Kitex RPC 服务。
//
// 使用 demoservice.NewServer 创建服务，绑定到 addr 地址。
// 如果 obsProvider 不为 nil，则注入 OTel 可观测性 Suite。
func StartServer(ctx context.Context, addr string, obsProvider *kitexobs.Provider) error {
	handler := &DemoServiceImpl{}

	var opts []server.Option
	opts = append(opts, server.WithServiceAddr(&net.TCPAddr{Port: extractPort(addr)}))

	// 注入 OTel 可观测性。
	if obsProvider != nil && obsProvider.Enabled() {
		suite := obsProvider.ServerSuite()
		opts = append(opts, suite.Options()...)
		klog.Infof("kitex observability enabled, addr=%s", addr)
	}

	svr := demoservice.NewServer(handler, opts...)

	log.L().Info("kitex server starting", "addr", addr)

	// svr.Run() blocks; run in goroutine so we can listen for ctx cancellation.
	errCh := make(chan error, 1)
	go func() {
		errCh <- svr.Run()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.L().Info("kitex server shutting down")
		_ = svr.Stop()
		return nil
	}
}

// extractPort 从 addr 字符串提取端口号（如 ":8888" → 8888）。
func extractPort(addr string) int {
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	port, _ := strconv.Atoi(portStr)
	return port
}
