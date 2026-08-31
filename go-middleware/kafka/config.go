// Package kafka 提供 Kafka 生产者和消费者配置及工厂方法（基于 kafka-go）。
package kafka

import "time"

// WriterConfig Kafka Writer（生产者）配置
type WriterConfig struct {
	Broker []string `json:"broker" yaml:"broker"`
	Topic  string   `json:"topic" yaml:"topic"`

	// AllowAutoTopicCreation 当 topic 不存在时自动创建。
	AllowAutoTopicCreation bool `json:"allow_auto_topic_creation" yaml:"allow_auto_topic_creation"`

	TLS struct {
		Enable             bool `json:"enable" yaml:"enable"`
		InsecureSkipVerify bool `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	} `json:"tls" yaml:"tls"`

	SASL struct {
		Enable   bool   `json:"enable" yaml:"enable"`
		User     string `json:"user" yaml:"user"`
		Password string `json:"password" yaml:"password"`
	} `json:"sasl" yaml:"sasl"`

	// MaxAttempts 单条消息写入失败后的最大重试次数（0 表示使用 kafka-go 默认值 10）。
	MaxAttempts int `json:"max_attempts" yaml:"max_attempts"`
	// WriteBackoffMin 重试前等待的最小退避时间（0 表示使用 kafka-go 默认值 100ms）。
	WriteBackoffMin time.Duration `json:"write_backoff_min" yaml:"write_backoff_min"`
	// WriteBackoffMax 重试前等待的最大退避时间（0 表示使用 kafka-go 默认值 1s）。
	WriteBackoffMax time.Duration `json:"write_backoff_max" yaml:"write_backoff_max"`
}

// ReaderConfig Kafka Reader（消费者）配置
type ReaderConfig struct {
	Broker  []string `json:"broker" yaml:"broker"`
	Topic   string   `json:"topic" yaml:"topic"`
	GroupID string   `json:"group_id" yaml:"group_id"`

	MinBytes         int           `json:"min_bytes" yaml:"min_bytes"`
	MaxBytes         int           `json:"max_bytes" yaml:"max_bytes"`
	MaxWait          time.Duration `json:"max_wait" yaml:"max_wait"`
	ReadBatchTimeout time.Duration `json:"read_batch_timeout" yaml:"read_batch_timeout"`

	TLS struct {
		Enable             bool `json:"enable" yaml:"enable"`
		InsecureSkipVerify bool `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	} `json:"tls" yaml:"tls"`

	SASL struct {
		Enable   bool   `json:"enable" yaml:"enable"`
		User     string `json:"user" yaml:"user"`
		Password string `json:"password" yaml:"password"`
	} `json:"sasl" yaml:"sasl"`

	// MaxAttempts 单次读取失败后的最大重试次数（0 表示使用 kafka-go 默认值 3）。
	MaxAttempts int `json:"max_attempts" yaml:"max_attempts"`
	// ReadBackoffMin 重试前等待的最小退避时间（0 表示使用 kafka-go 默认值 100ms）。
	ReadBackoffMin time.Duration `json:"read_backoff_min" yaml:"read_backoff_min"`
	// ReadBackoffMax 重试前等待的最大退避时间（0 表示使用 kafka-go 默认值 1s）。
	ReadBackoffMax time.Duration `json:"read_backoff_max" yaml:"read_backoff_max"`
}
