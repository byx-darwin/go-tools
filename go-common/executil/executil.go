// Package executil 提供增强的命令执行包装器。
package executil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// Runner 可 mock 的执行接口。
type Runner interface {
	Run(ctx context.Context, cmd *Cmd) *Result
}

// Cmd 命令配置。
type Cmd struct {
	Name     string
	Args     []string
	Dir      string
	Env      []string
	Stdin    io.Reader
	Timeout  time.Duration
	OnStdout func([]byte)
	OnStderr func([]byte)
}

// Result 执行结果。
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Err      error
}

type execRunner struct{}

// New 创建默认 Runner。
func New() Runner {
	return &execRunner{}
}

// Run 执行命令。
func (r *execRunner) Run(ctx context.Context, cmd *Cmd) *Result {
	if cmd.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cmd.Timeout)
		defer cancel()
	}

	c := exec.CommandContext(ctx, cmd.Name, cmd.Args...)
	if cmd.Dir != "" {
		c.Dir = cmd.Dir
	}
	if len(cmd.Env) > 0 {
		c.Env = cmd.Env
	}
	if cmd.Stdin != nil {
		c.Stdin = cmd.Stdin
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	var stdoutW, stderrW io.Writer = &stdoutBuf, &stderrBuf

	if cmd.OnStdout != nil {
		stdoutW = io.MultiWriter(&stdoutBuf, writerFunc(cmd.OnStdout))
	}
	if cmd.OnStderr != nil {
		stderrW = io.MultiWriter(&stderrBuf, writerFunc(cmd.OnStderr))
	}

	c.Stdout = stdoutW
	c.Stderr = stderrW

	err := c.Run()
	result := &Result{
		Stdout: stdoutBuf.Bytes(),
		Stderr: stderrBuf.Bytes(),
	}

	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result.Err = &TimeoutError{Duration: cmd.Timeout}
		} else if exitErr := new(exec.ExitError); errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
			result.Err = &ExitError{
				ExitCode: exitErr.ExitCode(),
				Stderr:   truncate(stderrBuf.Bytes(), 1024),
			}
		} else {
			result.Err = &NotFoundError{Name: cmd.Name}
		}
	}

	return result
}

type writerFunc func([]byte)

func (f writerFunc) Write(p []byte) (int, error) {
	f(p)
	return len(p), nil
}

func truncate(b []byte, maxLen int) []byte {
	if len(b) <= maxLen {
		return b
	}
	return b[:maxLen]
}

// NotFoundError 命令不存在错误。
type NotFoundError struct {
	Name string
	Hint string
}

func (e *NotFoundError) Error() string {
	msg := "command not found: " + e.Name
	if e.Hint != "" {
		msg += " (" + e.Hint + ")"
	}
	return msg
}

// ExitError 命令退出码非零错误。
type ExitError struct {
	ExitCode int
	Stderr   []byte
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("command exited with code %d", e.ExitCode)
}

// TimeoutError 命令执行超时错误。
type TimeoutError struct {
	Duration time.Duration
}

func (e *TimeoutError) Error() string {
	return "command timed out after " + e.Duration.String()
}
