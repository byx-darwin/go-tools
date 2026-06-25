//go:build with_rotation

package log

import (
	"io"
	"os"
)

// createFileWriter 创建带轮转的文件写入器（需要 build tag: with_rotation）。
func createFileWriter(cfg FileConfig) io.Writer {
	if cfg.Dir == "" || cfg.Filename == "" {
		return os.Stdout
	}
	return createRotationWriter(cfg)
}
