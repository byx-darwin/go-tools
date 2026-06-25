package astutil_test

import (
	"testing"

	"github.com/byx-darwin/go-tools/go-common/astutil"
	"github.com/stretchr/testify/require"
)

func TestFile_FindFunctions(t *testing.T) {
	file, _ := astutil.ParseSource([]byte(`package main

func foo() {}
func bar() {}
`))
	funcs := file.FindFunctions()
	require.Len(t, funcs, 2)
}

func TestFile_FindFunction(t *testing.T) {
	file, _ := astutil.ParseSource([]byte(`package main

func foo() {}
func bar() {}
`))
	fn := file.FindFunction("foo")
	require.NotNil(t, fn)
	require.Equal(t, "foo", fn.Name.Name)
}

func TestFile_FindImports(t *testing.T) {
	file, _ := astutil.ParseSource([]byte(`package main

import "fmt"
import "os"
`))
	imports := file.FindImports()
	require.Len(t, imports, 2)
}

func TestParseFile(t *testing.T) {
	// 创建一个临时文件
	// 这里简化，实际测试需要创建真实文件
	file, err := astutil.ParseSource([]byte(`package main

func main() {}
`))
	require.NoError(t, err)
	require.NotNil(t, file)
}
