// Package tls 提供日志文件到火山引擎 TLS 的自动上报。
//
// FileShipper 定时检查本地日志文件，读取新增的 JSON 行并批量上报。
//
// 用法:
//
//	shipper, _ := tls.NewFileShipper(tls.FileShipperConfig{
//	    ProducerConfig: tls.ProducerConfig{...},
//	    FilePath:       "/var/log/app.log",
//	})
//	shipper.Start()
//	defer shipper.Close()
package tls

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/samber/oops"
)

// FileShipperConfig 文件上报配置。
type FileShipperConfig struct {
	ProducerConfig ProducerConfig `json:"producer" yaml:"producer"`
	FilePath       string         `json:"file_path" yaml:"file_path"`
	CheckInterval  time.Duration  `json:"check_interval" yaml:"check_interval"`
	MaxLineSize    int            `json:"max_line_size" yaml:"max_line_size"`
}

// FileShipper 定时读取本地日志文件并批量上报到 TLS。
type FileShipper struct {
	producer *Producer
	config   FileShipperConfig
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewFileShipper 创建文件上报器。
func NewFileShipper(cfg FileShipperConfig) (*FileShipper, error) {
	if cfg.FilePath == "" {
		return nil, oops.With("tls.NewFileShipper").
			Code(CodeInvalidConfig).
			Errorf("file_path is required")
	}
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = 2 * time.Second
	}
	if cfg.MaxLineSize == 0 {
		cfg.MaxLineSize = 64 * 1024
	}
	producer, err := NewProducer(cfg.ProducerConfig)
	if err != nil {
		return nil, oops.With("tls.NewFileShipper").
			Code(CodeProducerInit).
			Wrap(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &FileShipper{producer: producer, config: cfg, ctx: ctx, cancel: cancel, done: make(chan struct{})}, nil
}

// Start 启动后台轮协程。
func (s *FileShipper) Start() { go s.run() }

// Close 停止轮询并关闭底层 Producer。
func (s *FileShipper) Close() error {
	s.cancel()
	select {
	case <-s.done:
	default:
	}
	return s.producer.Close()
}

func (s *FileShipper) run() {
	defer close(s.done)
	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	var offset int64
	if f, err := os.Open(s.config.FilePath); err == nil {
		fi, _ := f.Stat()
		offset = fi.Size()
		_ = f.Close()
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			n, err := s.shipSince(offset)
			if err == nil {
				offset = n
			}
		}
	}
}

// shipSince 从 offset 开始读取新增的完整日志行并上报。
//
// 逐行读取而不是一次性 Scan 完整个新增区间：一旦某行 SendLog 失败，
// 立即停止并返回该行之前的 offset，使这一行在下次 tick 时被重新读取，
// 避免像之前那样无条件推进到文件末尾、静默丢弃发送失败的日志行。
func (s *FileShipper) shipSince(offset int64) (int64, error) {
	f, err := os.Open(s.config.FilePath)
	if err != nil {
		return offset, err
	}
	defer func() { _ = f.Close() }()

	fi, err := f.Stat()
	if err != nil {
		return offset, err
	}
	if fi.Size() <= offset {
		return offset, nil
	}

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset, err
	}

	reader := bufio.NewReaderSize(f, s.config.MaxLineSize)
	pos := offset
	for {
		line, readErr := reader.ReadBytes('\n')
		if readErr == io.EOF {
			// 末尾不完整的行（可能仍在被写入），留到下次 tick 重新读取。
			break
		}
		if readErr != nil {
			return pos, readErr
		}

		fields := parseJSONLine(bytes.TrimRight(line, "\n"))
		if fields != nil {
			if err := s.producer.SendLog(s.ctx, fields); err != nil {
				return pos, nil
			}
		}
		pos += int64(len(line))
	}
	return pos, nil
}

func parseJSONLine(line []byte) map[string]string {
	var raw map[string]any
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil
	}
	fields := make(map[string]string, len(raw))
	for k, v := range raw {
		fields[k] = fmt.Sprint(v)
	}
	return fields
}
