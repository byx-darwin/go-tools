// go-common/executil/executil_test.go
package executil_test

import (
	"bytes"
	"context"
	"testing"
	"time"

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

func TestRun_NotFoundError(t *testing.T) {
	runner := executil.New()
	result := runner.Run(context.Background(), &executil.Cmd{
		Name: "nonexistent_command_12345",
	})
	require.Error(t, result.Err)
	var nfe *executil.NotFoundError
	require.ErrorAs(t, result.Err, &nfe)
	require.Equal(t, "nonexistent_command_12345", nfe.Name)
}

func TestRun_ExitError(t *testing.T) {
	runner := executil.New()
	result := runner.Run(context.Background(), &executil.Cmd{
		Name: "sh",
		Args: []string{"-c", "exit 42"},
	})
	require.Error(t, result.Err)
	var ee *executil.ExitError
	require.ErrorAs(t, result.Err, &ee)
	require.Equal(t, 42, ee.ExitCode)
}

func TestRun_TimeoutError(t *testing.T) {
	runner := executil.New()
	result := runner.Run(context.Background(), &executil.Cmd{
		Name:    "sleep",
		Args:    []string{"10"},
		Timeout: 100 * time.Millisecond,
	})
	require.Error(t, result.Err)
	var te *executil.TimeoutError
	require.ErrorAs(t, result.Err, &te)
	require.Equal(t, 100*time.Millisecond, te.Duration)
}

func TestRun_StreamingOutput(t *testing.T) {
	runner := executil.New()
	var buf bytes.Buffer
	result := runner.Run(context.Background(), &executil.Cmd{
		Name: "sh",
		Args: []string{"-c", "echo line1; echo line2"},
		OnStdout: func(line []byte) {
			buf.Write(line)
		},
	})
	require.NoError(t, result.Err)
	require.Contains(t, buf.String(), "line1")
	require.Contains(t, buf.String(), "line2")
}

func TestRun_ContextCancellation(t *testing.T) {
	runner := executil.New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消
	result := runner.Run(ctx, &executil.Cmd{
		Name: "sleep",
		Args: []string{"10"},
	})
	require.Error(t, result.Err)
}
