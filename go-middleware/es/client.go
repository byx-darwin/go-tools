package es

import (
	"crypto/tls"
	"net/http"

	elasticsearchv8 "github.com/elastic/go-elasticsearch/v8"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// NewClient 创建 Elasticsearch v8 客户端。
// 使用官方 go-elasticsearch 库，支持 TLS 和认证配置，可选 OpenTelemetry 追踪（WithTrace）。
func NewClient(config Config, opts ...ClientOption) (*elasticsearchv8.Client, error) {
	o := &clientOptions{}
	for _, opt := range opts {
		opt(o)
	}

	cfg := elasticsearchv8.Config{
		Addresses: config.Addresses,
		Username:  config.Username,
		Password:  config.Password,
		APIKey:    config.APIKey,
		CloudID:   config.CloudID,
		MaxRetries: func() int {
			if config.MaxRetries > 0 {
				return config.MaxRetries
			}
			return 3
		}(),
	}

	if config.MaxIdleConnsPerHost > 0 {
		cfg.Transport = &http.Transport{
			MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
		}
	}

	if config.TLS.Enable {
		if cfg.Transport == nil {
			cfg.Transport = &http.Transport{}
		}
		cfg.Transport.(*http.Transport).TLSClientConfig = &tls.Config{
			InsecureSkipVerify: config.TLS.InsecureSkipVerify, //nolint:gosec // 用户可通过配置显式关闭 TLS 校验
		}
	}

	if o.trace {
		cfg.Transport = otelhttp.NewTransport(cfg.Transport)
	}

	client, err := elasticsearchv8.NewClient(cfg)
	if err != nil {
		return nil, ErrInit.Wrap(err)
	}
	return client, nil
}
