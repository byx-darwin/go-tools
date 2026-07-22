// Package tls 提供火山引擎日志服务 (Tinder Log Service) 客户端。
//
// Producer 负责将结构化日志批量上报到火山引擎 TLS。
//
// 用法:
//
//	p, _ := tls.NewProducer(tls.ProducerConfig{
//	    Endpoint:        "tls-cn-beijing.volces.com",
//	    AccessKeyID:     os.Getenv("VOLC_AK"),
//	    AccessKeySecret: os.Getenv("VOLC_SK"),
//	    TopicID:         "your-topic-id",
//	})
//	defer p.Close()
//	p.SendLog(context.Background(), map[string]string{"level": "info", "msg": "hello"})
package tls

import (
	"context"
	"sync"
	"time"

	"github.com/samber/oops"
	"github.com/volcengine/volc-sdk-golang/service/tls"
)

// ProducerConfig 生产者配置
type ProducerConfig struct {
	// Endpoint TLS 服务地址（如 tls-cn-beijing.volces.com）
	Endpoint string `json:"endpoint" yaml:"endpoint"`
	// AccessKeyID 火山引擎 AK
	AccessKeyID string `json:"access_key_id" yaml:"access_key_id"`
	// AccessKeySecret 火山引擎 SK
	AccessKeySecret string `json:"access_key_secret" yaml:"access_key_secret"`
	// Region 区域（如 cn-beijing）
	Region string `json:"region" yaml:"region"`
	// TopicID 日志主题 ID
	TopicID string `json:"topic_id" yaml:"topic_id"`
	// Source 日志来源标识（默认 "go-tools"）
	Source string `json:"source" yaml:"source"`
	// BatchSize 批量发送大小（默认 10）
	BatchSize int `json:"batch_size" yaml:"batch_size"`
	// FlushInterval 刷新间隔（默认 5s）
	FlushInterval time.Duration `json:"flush_interval" yaml:"flush_interval"`
}

// Producer TLS 日志生产者。
type Producer struct {
	client  tls.Client
	config  ProducerConfig
	buf     []tls.Log
	mu      sync.Mutex
	closeCh chan struct{}
	done    chan struct{}
}

// NewProducer 创建 TLS Producer。
func NewProducer(cfg ProducerConfig) (*Producer, error) {
	if cfg.Endpoint == "" {
		return nil, oops.With("tls.NewProducer").
			Code(CodeInvalidConfig).
			Errorf("endpoint is required")
	}
	if cfg.TopicID == "" {
		return nil, oops.With("tls.NewProducer").
			Code(CodeInvalidConfig).
			Errorf("topic_id is required")
	}
	if cfg.Region == "" {
		return nil, oops.With("tls.NewProducer").
			Code(CodeInvalidConfig).
			Errorf("region is required")
	}
	if cfg.Source == "" {
		cfg.Source = "go-tools"
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 10
	}
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = 5 * time.Second
	}

	client := tls.NewClient(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret, "", cfg.Region)
	p := &Producer{
		client:  client,
		config:  cfg,
		buf:     make([]tls.Log, 0, cfg.BatchSize),
		closeCh: make(chan struct{}),
		done:    make(chan struct{}),
	}

	go p.flushLoop()
	return p, nil
}

// SendLog 发送单条日志（异步批量，非阻塞）。
func (p *Producer) SendLog(ctx context.Context, fields map[string]string) error {
	log := tls.Log{}
	for k, v := range fields {
		log.Contents = append(log.Contents, tls.LogContent{Key: k, Value: v})
	}

	p.mu.Lock()
	p.buf = append(p.buf, log)
	needFlush := len(p.buf) >= p.config.BatchSize
	p.mu.Unlock()

	if needFlush {
		return p.flush(ctx)
	}
	return nil
}

// SendLogs 发送多条日志（异步批量）。
func (p *Producer) SendLogs(ctx context.Context, entries []map[string]string) error {
	for _, fields := range entries {
		if err := p.SendLog(ctx, fields); err != nil {
			return err
		}
	}
	return nil
}

// Flush 强制刷新缓冲区。
func (p *Producer) Flush(ctx context.Context) error {
	return p.flush(ctx)
}

func (p *Producer) flush(_ context.Context) error {
	p.mu.Lock()
	if len(p.buf) == 0 {
		p.mu.Unlock()
		return nil
	}
	logs := p.buf
	p.buf = make([]tls.Log, 0, p.config.BatchSize)
	p.mu.Unlock()

	_, err := p.client.PutLogsV2(&tls.PutLogsV2Request{
		TopicID:      p.config.TopicID,
		CompressType: "lz4",
		Source:       p.config.Source,
		Logs:         logs,
	})
	if err != nil {
		return oops.With("tls.flush").
			Code(CodeSend).
			Wrap(err)
	}
	return nil
}

func (p *Producer) flushLoop() {
	ticker := time.NewTicker(p.config.FlushInterval)
	defer ticker.Stop()
	defer close(p.done)

	for {
		select {
		case <-p.closeCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = p.flush(ctx)
			cancel()
		}
	}
}

// Close 关闭 Producer。
func (p *Producer) Close() error {
	close(p.closeCh)
	<-p.done
	return nil
}
