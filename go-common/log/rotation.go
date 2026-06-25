//go:build with_rotation

package log

import (
	"io"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// createRotationWriter 创建带轮转的文件写入器（需要 build tag: with_rotation）。
func createRotationWriter(cfg FileConfig) io.WriteCloser {
	return &lumberjack.Logger{
		Filename:   filepath.Join(cfg.Dir, cfg.Filename),
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}
}
