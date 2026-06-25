// go-common/astutil/gen/gen_test.go
package gen_test

import (
	"testing"

	"github.com/byx-darwin/go-tools/go-common/astutil/gen"
	"github.com/stretchr/testify/require"
)

func TestNewFile(t *testing.T) {
	f := gen.NewFile("mypackage")
	require.NotNil(t, f)
}

func TestFile_Render(t *testing.T) {
	f := gen.NewFile("mypackage")
	code, err := f.Render()
	require.NoError(t, err)
	require.Contains(t, code, "package mypackage")
}

func TestFile_Func(t *testing.T) {
	f := gen.NewFile("mypackage")
	f.Func("main").
		Body()
	code, _ := f.Render()
	require.Contains(t, code, "func main()")
}

func TestFile_FuncWithParams(t *testing.T) {
	f := gen.NewFile("mypackage")
	f.Func("handle").
		Params(gen.Param("ctx", "context.Context")).
		Body()
	code, _ := f.Render()
	require.Contains(t, code, "func handle(ctx context.Context)")
}

func TestFile_Struct(t *testing.T) {
	f := gen.NewFile("mypackage")
	f.Struct("Config").
		Field("Name", "string").
		Field("Timeout", "time.Duration")
	code, _ := f.Render()
	require.Contains(t, code, "type Config struct")
	require.Contains(t, code, "Name string")
	require.Contains(t, code, "Timeout time.Duration")
}

func TestFile_StructWithTag(t *testing.T) {
	f := gen.NewFile("mypackage")
	f.Struct("Config").
		Field("Name", "string").Tag(`json:"name"`)
	code, _ := f.Render()
	require.Contains(t, code, `Name string `+"`"+`json:"name"`+"`")
}

func TestFile_FuncWithResults(t *testing.T) {
	f := gen.NewFile("mypackage")
	f.Func("getName").
		Results("string").
		Body(`return "hello"`)
	code, _ := f.Render()
	require.Contains(t, code, "func getName() (string)")
	require.Contains(t, code, `return "hello"`)
}

func TestFile_FuncWithMultipleResults(t *testing.T) {
	f := gen.NewFile("mypackage")
	f.Func("doWork").
		Params(gen.Param("ctx", "context.Context")).
		Results("string", "error").
		Body(`return "done", nil`)
	code, _ := f.Render()
	require.Contains(t, code, "func doWork(ctx context.Context) (string, error)")
	require.Contains(t, code, `return "done", nil`)
}

func TestFile_FuncFull(t *testing.T) {
	f := gen.NewFile("mypackage")
	f.Func("handle").
		Params(gen.Param("w", "http.ResponseWriter"), gen.Param("r", "*http.Request")).
		Results("error").
		Body(
			"w.WriteHeader(200)",
			`return nil`,
		)
	code, _ := f.Render()
	require.Contains(t, code, "func handle(w http.ResponseWriter, r *http.Request) (error)")
	require.Contains(t, code, "w.WriteHeader(200)")
	require.Contains(t, code, "return nil")
}

func TestFile_RenderWithImports(t *testing.T) {
	f := gen.NewFile("mypackage")
	f.Import("context")
	f.Import("time")
	f.Func("doSomething").
		Params(gen.Param("ctx", "context.Context"), gen.Param("d", "time.Duration")).
		Body()
	code, err := f.Render()
	require.NoError(t, err)
	require.Contains(t, code, "import (")
	require.Contains(t, code, `"context"`)
	require.Contains(t, code, `"time"`)
	require.Contains(t, code, "func doSomething(ctx context.Context, d time.Duration)")
}

func TestFile_RenderWithImportAndResults(t *testing.T) {
	f := gen.NewFile("mypackage")
	f.Import("errors")
	f.Func("validate").
		Params(gen.Param("s", "string")).
		Results("error").
		Body(
			`if s == "" {`,
			`	return errors.New("empty string")`,
			`}`,
			"return nil",
		)
	code, err := f.Render()
	require.NoError(t, err)
	require.Contains(t, code, `"errors"`)
	require.Contains(t, code, `func validate(s string) (error)`)
	require.Contains(t, code, `errors.New("empty string")`)
}
