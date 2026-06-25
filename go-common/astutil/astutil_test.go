package astutil_test

import (
	"os"
	"path/filepath"
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

func TestFile_AddImport(t *testing.T) {
	file, _ := astutil.ParseSource([]byte(`package main
`))
	file.Apply(astutil.AddImport("fmt"))
	imports := file.FindImports()
	require.Len(t, imports, 1)
	require.Contains(t, imports[0].Path.Value, "fmt")
}

func TestFile_AddImport_Idempotent(t *testing.T) {
	file, _ := astutil.ParseSource([]byte(`package main

import "fmt"
`))
	file.Apply(astutil.AddImport("fmt"))
	imports := file.FindImports()
	require.Len(t, imports, 1) // 不会重复添加
}

func TestFile_RemoveImport(t *testing.T) {
	file, _ := astutil.ParseSource([]byte(`package main

import "fmt"
import "os"
`))
	file.Apply(astutil.RemoveImport("fmt"))
	imports := file.FindImports()
	require.Len(t, imports, 1)
	require.Contains(t, imports[0].Path.Value, "os")
}

func TestFile_Format(t *testing.T) {
	file, _ := astutil.ParseSource([]byte(`package main
import"fmt"
func main(){fmt.Println("hello")}
`))
	out, err := file.Format()
	require.NoError(t, err)
	require.Contains(t, string(out), "package main")
	require.Contains(t, string(out), `import "fmt"`)
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

func TestParseSource_Error(t *testing.T) {
	_, err := astutil.ParseSource([]byte(`this is not valid go code {{{`))
	require.Error(t, err)
}

func TestFile_FindFunction_NotFound(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

func foo() {}
`))
	require.NoError(t, err)
	fn := file.FindFunction("nonexistent")
	require.Nil(t, fn)
}

func TestFile_AddImport_WithExistingImports(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

import "fmt"
`))
	require.NoError(t, err)
	file.Apply(astutil.AddImport("os"))
	imports := file.FindImports()
	require.Len(t, imports, 2)
}

func TestParseFile_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	err := os.WriteFile(path, []byte(`package main

func hello() {}
`), 0o644)
	require.NoError(t, err)

	file, err := astutil.ParseFile(path)
	require.NoError(t, err)
	require.NotNil(t, file)
	fn := file.FindFunction("hello")
	require.NotNil(t, fn)
}

func TestFile_WriteTo(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

func main() {}
`))
	require.NoError(t, err)

	dir := t.TempDir()
	path := filepath.Join(dir, "output.go")
	err = file.WriteTo(path)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), "package main")
	require.Contains(t, string(data), "func main()")
}

func TestFile_WriteTo_Error(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main
`))
	require.NoError(t, err)

	dir := t.TempDir()
	err = file.WriteTo(dir) // 写入目录路径应失败
	require.Error(t, err)
}
