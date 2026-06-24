package clickhouse

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// NewClient 创建 ClickHouse 原生协议客户端。
// 使用 clickhouse-go v2 原生接口，支持 DSN 和字段两种配置方式。
func NewClient(config Config) (clickhouse.Conn, error) {
	// If DSN is provided, use it directly
	if config.DSN != "" {
		opts, err := clickhouse.ParseDSN(config.DSN)
		if err != nil {
			return nil, fmt.Errorf("clickhouse: parse DSN: %w", err)
		}
		return clickhouse.Open(opts)
	}

	opts := &clickhouse.Options{
		Addr: config.Addrs,
		Auth: clickhouse.Auth{
			Database: config.Database,
			Username: config.Username,
			Password: config.Password,
		},
		Compression: &clickhouse.Compression{
			Method: func() clickhouse.CompressionMethod {
				if config.Compress {
					return clickhouse.CompressionLZ4
				}
				return clickhouse.CompressionNone
			}(),
		},
	}

	if config.DialTimeout > 0 {
		opts.DialTimeout = sec(config.DialTimeout)
	} else {
		opts.DialTimeout = 10 * time.Second
	}

	if config.MaxOpenConns > 0 {
		opts.MaxOpenConns = config.MaxOpenConns
	}
	if config.MaxIdleConns > 0 {
		opts.MaxIdleConns = config.MaxIdleConns
	}
	if config.ConnMaxLifetime > 0 {
		opts.ConnMaxLifetime = sec(config.ConnMaxLifetime)
	}

	if config.TLS.Enable {
		opts.TLS = &tls.Config{
			InsecureSkipVerify: config.TLS.InsecureSkipVerify, //nolint:gosec // 用户可通过配置显式关闭 TLS 校验
		}
	}

	return clickhouse.Open(opts)
}
