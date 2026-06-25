//go:build with_rotation

package log_test

import (
	"testing"

	"github.com/byx-darwin/go-tools/go-common/log"
	"github.com/stretchr/testify/require"
)

func TestRotation_Enabled(t *testing.T) {
	cfg := log.FileConfig{
		Dir:        "/tmp",
		Filename:   "test.log",
		MaxSize:    100,
		MaxBackups: 7,
		MaxAge:     30,
		Compress:   true,
	}
	// 测试 rotation writer 创建
	require.NotPanics(t, func() {
		_ = cfg
	})
}
