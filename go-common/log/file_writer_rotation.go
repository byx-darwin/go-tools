//go:build with_rotation

package log

import "io"

// createFileWriter 创建带轮转的文件写入器（需要 build tag: with_rotation）。
func createFileWriter(cfg FileConfig) io.Writer {
	if cfg.Dir == "" || cfg.Filename == "" {
		return nil
	}
	return createRotationWriter(cfg)
}
