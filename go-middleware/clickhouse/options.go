package clickhouse

// ClientOption 定义 ClickHouse 客户端创建选项。
type ClientOption func(*clientOptions)

type clientOptions struct {
	trace bool
}

// WithTrace 启用 OpenTelemetry 查询追踪。
func WithTrace() ClientOption {
	return func(o *clientOptions) {
		o.trace = true
	}
}
