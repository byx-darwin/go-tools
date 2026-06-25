//go:build !with_rotation

package log

import (
	"fmt"
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
		fmt.Fprintf(os.Stderr, "[log] warning: failed to open log file %s/%s: %v; falling back to stdout\n", cfg.Dir, cfg.Filename, err)
		return os.Stdout
	}
	return f
}
