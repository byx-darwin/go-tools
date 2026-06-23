package kafka

import "time"

// WriterConfig Kafka Writer（生产者）配置
type WriterConfig struct {
	Broker  []string `json:"broker" yaml:"broker"`
	Topic   string   `json:"topic" yaml:"topic"`

	TLS struct {
		Enable             bool `json:"enable" yaml:"enable"`
		InsecureSkipVerify bool `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	} `json:"tls" yaml:"tls"`

	SASL struct {
		Enable   bool   `json:"enable" yaml:"enable"`
		User     string `json:"user" yaml:"user"`
		Password string `json:"password" yaml:"password"`
	} `json:"sasl" yaml:"sasl"`
}

// ReaderConfig Kafka Reader（消费者）配置
type ReaderConfig struct {
	Broker  []string `json:"broker" yaml:"broker"`
	Topic   string   `json:"topic" yaml:"topic"`
	GroupID string   `json:"group_id" yaml:"group_id"`

	MinBytes          int           `json:"min_bytes" yaml:"min_bytes"`
	MaxBytes          int           `json:"max_bytes" yaml:"max_bytes"`
	MaxWait           time.Duration `json:"max_wait" yaml:"max_wait"`
	ReadBatchTimeout  time.Duration `json:"read_batch_timeout" yaml:"read_batch_timeout"`

	TLS struct {
		Enable             bool `json:"enable" yaml:"enable"`
		InsecureSkipVerify bool `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	} `json:"tls" yaml:"tls"`

	SASL struct {
		Enable   bool   `json:"enable" yaml:"enable"`
		User     string `json:"user" yaml:"user"`
		Password string `json:"password" yaml:"password"`
	} `json:"sasl" yaml:"sasl"`
}
