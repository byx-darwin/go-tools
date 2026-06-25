//go:build !with_rotation

package log

import (
	"io"
	"os"
	"path/filepath"
)

// createFileWriter 创建简单的文件写入器（无轮转）。
func createFileWriter(cfg FileConfig) io.Writer {
	if cfg.Dir == "" || cfg.Filename == "" {
		return os.Stdout
	}
	_ = os.MkdirAll(cfg.Dir, 0o755)
	f, err := os.OpenFile(filepath.Join(cfg.Dir, cfg.Filename), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return os.Stdout
	}
	return f
}
