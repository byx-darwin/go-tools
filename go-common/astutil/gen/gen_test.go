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
