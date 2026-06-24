package observability

import (
	"context"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/transmeta"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/transport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/byx-darwin/go-tools/go-framework/config"
)

var (
	_ client.Suite = (*clientSuite)(nil)
	_ server.Suite = (*serverSuite)(nil)
)

// TracerOption 定义 tracer 的可选配置。
type TracerOption func(*tracerConfig)

type tracerConfig struct {
	recordSourceOperation bool
	enableGRPCMetadata    bool
}

// WithRecordSourceOperation 启用记录调用方操作名（source_operation 维度）。
// 注意：可能造成高基数问题，仅在需要服务拓扑时开启。
func WithRecordSourceOperation(enable bool) TracerOption {
	return func(c *tracerConfig) {
		c.recordSourceOperation = enable
	}
}

// WithEnableGRPCMetadata 启用 gRPC metadata 的 trace context 传播。
// 当服务使用 transport.GRPC 协议时需开启此选项。
func WithEnableGRPCMetadata() TracerOption {
	return func(c *tracerConfig) {
		c.enableGRPCMetadata = true
	}
}

// ── Server Suite ──

type serverSuite struct {
	sOpts []server.Option
}

// Options 实现 server.Suite 接口。
func (s *serverSuite) Options() []server.Option {
	return s.sOpts
}

// NewServerSuite 创建 Kitex 服务端 OTel Suite。
//
// 包含：
//   - stats.Tracer（RPC 生命周期 span + metrics）
//   - ServerMiddleware（span 创建 + trace context 提取 + peer service 提取）
//   - TTHeader meta handler（trace context 传播）
func NewServerSuite(cfg config.ObservabilityConfig, tracerOpts ...TracerOption) *serverSuite {
	tCfg := newTracerConfig(tracerOpts)

	st := &serverTracer{
		cfg:            cfg,
		tracer:         otel.GetTracerProvider().Tracer(instrumentationName),
		recordSourceOp: tCfg.recordSourceOperation,
	}

	sOpts := []server.Option{
		server.WithTracer(st),
		server.WithMiddleware(ServerMiddleware(cfg)),
		server.WithMetaHandler(transmeta.ServerTTHeaderHandler),
	}

	return &serverSuite{sOpts: sOpts}
}

// ── Client Suite ──

type clientSuite struct {
	cOpts []client.Option
}

// Options 实现 client.Suite 接口。
func (c *clientSuite) Options() []client.Option {
	return c.cOpts
}

// NewClientSuite 创建 Kitex 客户端 OTel Suite。
//
// 包含：
//   - stats.Tracer（客户端 RPC 生命周期 span + metrics）
//   - ClientMiddleware（trace context 注入 + peer service 注入）
//   - TTHeader meta handler（trace context 传播）
func NewClientSuite(cfg config.ObservabilityConfig, tracerOpts ...TracerOption) *clientSuite {
	tCfg := newTracerConfig(tracerOpts)

	ct := &clientTracer{
		cfg:            cfg,
		tracer:         otel.GetTracerProvider().Tracer(instrumentationName),
		recordSourceOp: tCfg.recordSourceOperation,
	}

	cOpts := []client.Option{
		client.WithTracer(ct),
		client.WithMiddleware(ClientMiddleware(cfg)),
		client.WithTransportProtocol(transport.TTHeader),
		client.WithMetaHandler(transmeta.ClientTTHeaderHandler),
	}

	return &clientSuite{cOpts: cOpts}
}

func newTracerConfig(opts []TracerOption) *tracerConfig {
	cfg := &tracerConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// ── ServerMiddleware ──

// ServerMiddleware 在服务端创建 span，从 incoming TTHeader + gRPC metadata 中提取
// trace context 和 peer service 属性。
func ServerMiddleware(cfg config.ObservabilityConfig) endpoint.Middleware {
	if !cfg.Enabled {
		return endpoint.DummyMiddleware
	}

	tracer := otel.GetTracerProvider().Tracer(instrumentationName)
	propagator := otel.GetTextMapPropagator()

	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) (err error) {
			tc := traceCarrierFromContext(ctx)
			if tc == nil {
				return next(ctx, req, resp)
			}

			// 从 metainfo 提取 incoming trace context
			md := metainfo.GetAllValues(ctx)
			ctx = Extract(ctx, propagator, md)

			// 提取 peer service 属性（TTHeader）
			peerAttrs := extractPeerServiceAttributesFromMetaInfo(md)

			// gRPC metadata 支持
			if cfg.EnableGRPCMetadata {
				if grpcMd, ok := metadata.FromIncomingContext(ctx); ok {
					ctx = extractGRPCMetadata(ctx, grpcMd)
					peerAttrs = append(peerAttrs, extractPeerServiceAttributesFromGRPCMetadata(grpcMd)...)
				}
			}

			ctx, span := tracer.Start(ctx, "rpc.server",
				oteltrace.WithSpanKind(oteltrace.SpanKindServer),
			)

			span.SetAttributes(peerAttrs...)
			tc.SetSpan(span)

			err = next(ctx, req, resp)
			// 不在此处 RecordError——serverTracer.Finish 会通过 st.Error() 统一处理
			return err
		}
	}
}

// ClientMiddleware 在客户端创建 span，将 trace context 和当前服务的 resource
// 信息（peer service）注入到 outgoing TTHeader + gRPC metadata。
func ClientMiddleware(cfg config.ObservabilityConfig) endpoint.Middleware {
	if !cfg.Enabled {
		return endpoint.DummyMiddleware
	}

	tracer := otel.GetTracerProvider().Tracer(instrumentationName)
	propagator := otel.GetTextMapPropagator()

	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) (err error) {
			span := oteltrace.SpanFromContext(ctx)
			if !span.IsRecording() {
				return next(ctx, req, resp)
			}

			// 从 parent span 获取当前服务的 resource attributes
			var resourceAttrs []attribute.KeyValue
			if readOnlySpan, ok := span.(trace.ReadOnlySpan); ok {
				resourceAttrs = readOnlySpan.Resource().Attributes()
			}

			// 注入 trace context 到 TTHeader metadata
			md := make(map[string]string)
			Inject(ctx, propagator, md)
			injectPeerServiceToMetaInfo(ctx, resourceAttrs)

			// 注入 trace context 到 gRPC metadata（如果启用）
			if cfg.EnableGRPCMetadata {
				if grpcMd, ok := metadata.FromOutgoingContext(ctx); ok {
					ctx = injectGRPCMetadata(ctx, grpcMd)
				}
			}

			// 创建 client span
			ctx, span = tracer.Start(ctx, "rpc.client",
				oteltrace.WithSpanKind(oteltrace.SpanKindClient),
			)

			// 将 span 写入 TraceCarrier，供 clientTracer.Finish 使用
			if tc := traceCarrierFromContext(ctx); tc != nil {
				tc.SetSpan(span)
			}

			// 将 TTHeader trace context 写入 metainfo
			for k, v := range md {
				ctx = metainfo.WithValue(ctx, k, v)
			}

			err = next(ctx, req, resp)
			// 不在此处 RecordError——clientTracer.Finish 会通过 st.Error() 统一处理
			return err
		}
	}
}
