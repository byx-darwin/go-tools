// go-common/executil/executil_test.go
package executil_test

import (
	"context"
	"testing"

	"github.com/byx-darwin/go-tools/go-common/executil"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	runner := executil.New()
	require.NotNil(t, runner)
}

func TestRun_SimpleCommand(t *testing.T) {
	runner := executil.New()
	result := runner.Run(context.Background(), &executil.Cmd{
		Name: "echo",
		Args: []string{"hello"},
	})
	require.NoError(t, result.Err)
	require.Equal(t, 0, result.ExitCode)
	require.Contains(t, string(result.Stdout), "hello")
}
