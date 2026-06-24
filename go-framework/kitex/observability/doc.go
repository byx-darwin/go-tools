// Package observability 提供 Kitex RPC 服务的 OpenTelemetry 可观测性集成。
//
// # 快速开始
//
// 推荐使用 Suite 模式一键接入：
//
//	import "github.com/byx-darwin/go-tools/go-framework/kitex/observability"
//
//	p, _ := observability.NewProvider(ctx, config.ObservabilityConfig{
//	    Enabled:       true,
//	    Endpoint:      "otelcol:4317",
//	    ServiceName:   "my-service",
//	    EnableMetrics: true,
//	})
//	defer p.Shutdown(ctx)
//
//	svr := echo.NewServer(
//	    new(EchoImpl),
//	    server.WithSuite(p.ServerSuite()),
//	)
//
// # 架构
//
// Provider 初始化双通道：
//   - Tracing:  OTLP gRPC exporter → TracerProvider → W3C TraceContext + B3 propagator
//   - Metrics:  OTLP gRPC exporter → MeterProvider → rpc.server.duration + Go runtime metrics
//
// stats.Tracer 深度集成 Kitex RPC 生命周期，采集 10+ 维度元数据：
//   - rpc.method, rpc.service, rpc.system
//   - peer.service, peer.namespace（自动拓扑图）
//   - rpc.kitex.protocol, rpc.kitex.recv_size, rpc.kitex.send_size
//
// # 组件
//
//   - Provider:   TracerProvider + MeterProvider 初始化/关闭
//   - serverTracer / clientTracer: 实现 Kitex stats.Tracer 接口
//   - NewServerSuite / NewClientSuite: 一键接入（tracer + middleware + meta handler）
//   - ServerMiddleware: 提取 trace context + peer service 属性
//   - ClientMiddleware: 注入 trace context + peer service 属性
package observability
