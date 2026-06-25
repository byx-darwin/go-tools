// go-common/templateutil/templateutil_test.go
package templateutil_test

import (
	"testing"

	"github.com/byx-darwin/go-tools/go-common/templateutil"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	reg := templateutil.NewRegistry()
	require.NotNil(t, reg)
}

func TestRender_SimpleTemplate(t *testing.T) {
	out, err := templateutil.Render("Hello {{ .Name }}", map[string]any{"Name": "World"})
	require.NoError(t, err)
	require.Equal(t, "Hello World", out)
}

func TestDefault_ToLower(t *testing.T) {
	out, err := templateutil.Render("{{ .Name | ToLower }}", map[string]any{"Name": "HELLO"})
	require.NoError(t, err)
	require.Equal(t, "hello", out)
}

func TestDefault_ToUpper(t *testing.T) {
	out, err := templateutil.Render("{{ .Name | ToUpper }}", map[string]any{"Name": "hello"})
	require.NoError(t, err)
	require.Equal(t, "HELLO", out)
}

func TestDefault_LowerFirst(t *testing.T) {
	out, err := templateutil.Render("{{ .Name | LowerFirst }}", map[string]any{"Name": "Hello"})
	require.NoError(t, err)
	require.Equal(t, "hello", out)
}

func TestDefault_UpperFirst(t *testing.T) {
	out, err := templateutil.Render("{{ .Name | UpperFirst }}", map[string]any{"Name": "hello"})
	require.NoError(t, err)
	require.Equal(t, "Hello", out)
}

func TestDefault_ToCamel(t *testing.T) {
	out, err := templateutil.Render("{{ .Name | ToCamel }}", map[string]any{"Name": "hello_world"})
	require.NoError(t, err)
	require.Equal(t, "helloWorld", out)
}

func TestDefault_ToSnake(t *testing.T) {
	out, err := templateutil.Render("{{ .Name | ToSnake }}", map[string]any{"Name": "HelloWorld"})
	require.NoError(t, err)
	require.Equal(t, "hello_world", out)
}

func TestDefault_ExportName(t *testing.T) {
	out, err := templateutil.Render("{{ .Name | ExportName }}", map[string]any{"Name": "hello"})
	require.NoError(t, err)
	require.Equal(t, "Hello", out)
}

func TestDefault_PrivateName(t *testing.T) {
	out, err := templateutil.Render("{{ .Name | PrivateName }}", map[string]any{"Name": "Hello"})
	require.NoError(t, err)
	require.Equal(t, "hello", out)
}

func TestRegistry_CustomFunction(t *testing.T) {
	reg := templateutil.NewRegistry().
		Register("double", func(s string) string { return s + s })

	out, err := templateutil.RenderWith("{{ .Name | double }}", map[string]any{"Name": "hi"}, reg)
	require.NoError(t, err)
	require.Equal(t, "hihi", out)
}

func TestRegistry_DefaultAndCustom(t *testing.T) {
	reg := templateutil.NewRegistry().
		Default().
		Register("exclaim", func(s string) string { return s + "!" })

	out, err := templateutil.RenderWith("{{ .Name | ToLower | exclaim }}", map[string]any{"Name": "HELLO"}, reg)
	require.NoError(t, err)
	require.Equal(t, "hello!", out)
}
