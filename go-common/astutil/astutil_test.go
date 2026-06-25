package astutil_test

import (
	"testing"

	"github.com/byx-darwin/go-tools/go-common/astutil"
	"github.com/stretchr/testify/require"
)

func TestParseFile(t *testing.T) {
	// 创建一个临时文件
	// 这里简化，实际测试需要创建真实文件
	file, err := astutil.ParseSource([]byte(`package main

func main() {}
`))
	require.NoError(t, err)
	require.NotNil(t, file)
}
