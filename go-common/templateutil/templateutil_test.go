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
