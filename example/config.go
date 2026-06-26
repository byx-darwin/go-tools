package main

import (
	"fmt"
	"os"

	"github.com/byx-darwin/go-tools/go-common/log"
	"github.com/byx-darwin/go-tools/go-framework/config"
	hertzConfig "github.com/byx-darwin/go-tools/go-framework/config/hertz"
	kitexConfig "github.com/byx-darwin/go-tools/go-framework/config/kitex"
	"github.com/byx-darwin/go-tools/go-middleware/clickhouse"
	"github.com/byx-darwin/go-tools/go-middleware/db"
	"github.com/byx-darwin/go-tools/go-middleware/es"
	"github.com/byx-darwin/go-tools/go-middleware/kafka"
	"github.com/byx-darwin/go-tools/go-middleware/redis"
	"gopkg.in/yaml.v3"
)

// AppConfig 应用全局配置，聚合 go-tools 各库的配置。
type AppConfig struct {
	// Server 服务地址配置（HTTP / RPC 端口）。
	Server ServerConfig `yaml:"server"`

	// Log 日志配置（go-common/log）。
	Log log.Config `yaml:"log"`

	// JWT JWT 鉴权配置。
	JWT JWTConfig `yaml:"jwt"`

	// StoreMode Session/Device 存储模式：memory 或 redis。
	StoreMode string `yaml:"store_mode"`

	// Redis Redis 客户端配置（go-middleware/redis）。
	Redis redis.Config `yaml:"redis"`

	// Kafka Kafka Writer（生产者）配置（go-middleware/kafka）。
	Kafka kafka.WriterConfig `yaml:"kafka"`

	// KafkaReader Kafka Reader（消费者）配置（go-middleware/kafka）。
	KafkaReader kafka.ReaderConfig `yaml:"kafka_reader"`

	// DB 数据库配置（go-middleware/db）。
	DB db.Config `yaml:"db"`

	// Elasticsearch Elasticsearch 客户端配置（go-middleware/es）。
	Elasticsearch es.Config `yaml:"elasticsearch"`

	// ClickHouse ClickHouse 客户端配置（go-middleware/clickhouse）。
	ClickHouse clickhouse.Config `yaml:"clickhouse"`

	// Captcha 图形验证码配置（go-framework/config）。
	Captcha config.CaptchaOption `yaml:"captcha"`

	// Hertz Hertz HTTP 服务配置（go-framework/config/hertz）。
	Hertz hertzConfig.ServerConfig `yaml:"hertz"`

	// Kitex Kitex RPC 服务配置（go-framework/config/kitex）。
	Kitex kitexConfig.ServerConfig `yaml:"kitex"`

	// Observability 可观测性配置（go-framework/config）。
	Observability config.ObservabilityConfig `yaml:"observability"`

	// Polaris 北极星配置中心。
	Polaris PolarisConfig `yaml:"polaris"`
}

// ServerConfig 服务地址配置。
type ServerConfig struct {
	// HTTPAddr HTTP 服务监听地址。
	HTTPAddr string `yaml:"http_addr"`

	// RPCAddr RPC 服务监听地址。
	RPCAddr string `yaml:"rpc_addr"`
}

// JWTConfig JWT 鉴权配置。
type JWTConfig struct {
	// Secret JWT 签名密钥。
	Secret string `yaml:"secret"`

	// Issuer JWT 签发者。
	Issuer string `yaml:"issuer"`

	// AccessExpiration Access Token 过期时间（支持 30s / 5m 格式）。
	AccessExpiration config.Duration `yaml:"access_expiration"`

	// RefreshExpiration Refresh Token 过期时间（支持 24h / 7d 格式）。
	RefreshExpiration config.Duration `yaml:"refresh_expiration"`
}

// PolarisConfig 北极星配置中心。
type PolarisConfig struct {
	// Enabled 是否启用北极星配置。
	Enabled bool `yaml:"enabled"`

	// Namespace 北极星命名空间。
	Namespace string `yaml:"namespace"`

	// FileGroup 配置文件组。
	FileGroup string `yaml:"file_group"`

	// FileName 配置文件名。
	FileName string `yaml:"file_name"`
}

// LoadConfig 从 YAML 文件加载配置并展开环境变量。
//
// 环境变量展开使用 os.ExpandEnv，支持 ${VAR} 和 ${VAR:-default} 语法。
func LoadConfig(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	// 展开环境变量（如 ${OTEL_ENDPOINT}、${REDIS_ADDR:-localhost:6379}）
	expanded := os.ExpandEnv(string(data))
	var cfg AppConfig
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}
