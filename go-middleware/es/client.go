package es

import (
	"crypto/tls"
	"net/http"

	elasticsearchv8 "github.com/elastic/go-elasticsearch/v8"
)

// NewClient 创建 Elasticsearch v8 客户端。
// 使用官方 go-elasticsearch 库，支持 TLS 和认证配置。
func NewClient(config Config) (*elasticsearchv8.Client, error) {
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
			InsecureSkipVerify: config.TLS.InsecureSkipVerify, //nolint:gosec
		}
	}

	return elasticsearchv8.NewClient(cfg)
}
