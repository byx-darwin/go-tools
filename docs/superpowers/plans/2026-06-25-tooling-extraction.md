# 工具类提取实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 ncgo 中的 3 个通用工具模块（AST 操作、命令执行、模板辅助）提取到 go-common，作为独立包发布。

**Architecture:** 创建 4 个独立包：`templateutil`（可插拔模板函数）、`executil`（增强命令执行）、`astutil`（基于 dave/dst 的 AST 操作）、`astutil/gen`（jennifer 风格代码生成）。每个包零耦合，按需引入。

**Tech Stack:** Go 1.25+, github.com/dave/dst, 标准库 os/exec, text/template

## Global Constraints

- Go 版本：1.25+（workspace 模式）
- 模块路径：`github.com/byx-darwin/go-tools/go-common`
- 外部依赖：仅 `astutil` 使用 `github.com/dave/dst`
- 代码风格：遵循 `.claude/rules/go.md`（gofmt, goimports, revive, errcheck, gocritic）
- 测试：每个包必须有单元测试，覆盖率 > 80%
- 文档：所有导出符号必须有 godoc 注释

---

## 文件结构

```
go-common/
├── templateutil/
│   ├── templateutil.go       ← Registry, Render, 默认函数集
│   └── templateutil_test.go  ← 单元测试
├── executil/
│   ├── executil.go           ← Runner, Cmd, Result, 错误类型
│   └── executil_test.go      ← 单元测试
├── astutil/
│   ├── astutil.go            ← ParseFile, ParseSource, File 类型
│   ├── query.go              ← Find* 查询方法
│   ├── ops.go                ← AddImport, Insert, Replace 等操作
│   ├── format.go             ← Format, WriteTo 输出
│   └── astutil_test.go       ← 单元测试
│   └── gen/
│       ├── gen.go            ← File, Func, Struct 构建器
│       ├── stmt.go           ← 语句构建器
│       ├── expr.go           ← 表达式构建器
│       └── gen_test.go       ← 单元测试
```

---

## 阶段 1：`templateutil`（最简单，无外部依赖）

### Task 1.1: 创建 templateutil 包骨架

**Files:**
- Create: `go-common/templateutil/templateutil.go`
- Create: `go-common/templateutil/templateutil_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `Registry`, `NewRegistry()`, `FuncMap()`, `Render()`, `RenderWith()`

- [ ] **Step 1: 写失败的测试**

```go
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
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd go-common && go test ./templateutil/... -v
```

Expected: FAIL — `package github.com/byx-darwin/go-tools/go-common/templateutil is not found`

- [ ] **Step 3: 创建最小实现**

```go
// go-common/templateutil/templateutil.go
// Package templateutil 提供可插拔的模板辅助函数库。
package templateutil

import (
    "bytes"
    "text/template"
)

// Registry 可插拔的函数注册器。
type Registry struct {
    funcs template.FuncMap
}

// NewRegistry 创建空注册器。
func NewRegistry() *Registry {
    return &Registry{
        funcs: make(template.FuncMap),
    }
}

// Register 注册单个函数。
func (r *Registry) Register(name string, fn any) *Registry {
    r.funcs[name] = fn
    return r
}

// RegisterAll 批量注册。
func (r *Registry) RegisterAll(funcs template.FuncMap) *Registry {
    for k, v := range funcs {
        r.funcs[k] = v
    }
    return r
}

// FuncMap 返回已注册的所有函数。
func (r *Registry) FuncMap() template.FuncMap {
    return r.funcs
}

// Render 使用默认函数集渲染模板。
func Render(tmpl string, data any) (string, error) {
    reg := NewRegistry().Default()
    return RenderWith(tmpl, data, reg)
}

// RenderWith 使用自定义 Registry 渲染。
func RenderWith(tmpl string, data any, reg *Registry) (string, error) {
    t, err := template.New("").Funcs(reg.FuncMap()).Parse(tmpl)
    if err != nil {
        return "", err
    }
    var buf bytes.Buffer
    if err := t.Execute(&buf, data); err != nil {
        return "", err
    }
    return buf.String(), nil
}

// Default 返回内置默认函数集。
func (r *Registry) Default() *Registry {
    // TODO: 添加默认函数
    return r
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd go-common && go test ./templateutil/... -v
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-common/templateutil/
git commit -m "feat(templateutil): add Registry and Render functions"
```

---

### Task 1.2: 实现默认函数集

**Files:**
- Modify: `go-common/templateutil/templateutil.go:60-65`
- Modify: `go-common/templateutil/templateutil_test.go`

**Interfaces:**
- Consumes: `Registry`
- Produces: `Default()` 方法返回包含 ToLower, ToUpper, LowerFirst, UpperFirst, ToCamel, ToSnake, ToKebab, ExportName, PrivateName 的函数集

- [ ] **Step 1: 写失败的测试**

```go
// 在 templateutil_test.go 添加
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
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd go-common && go test ./templateutil/... -v
```

Expected: FAIL — `ToLower`, `ToUpper` 等函数未定义

- [ ] **Step 3: 实现默认函数**

```go
// 在 templateutil.go 的 Default() 方法中替换 TODO
func (r *Registry) Default() *Registry {
    r.Register("ToLower", strings.ToLower)
    r.Register("ToUpper", strings.ToUpper)
    r.Register("LowerFirst", lowerFirst)
    r.Register("UpperFirst", upperFirst)
    r.Register("ToCamel", toCamel)
    r.Register("ToSnake", toSnake)
    r.Register("ToKebab", toKebab)
    r.Register("ExportName", upperFirst)
    r.Register("PrivateName", lowerFirst)
    return r
}

func lowerFirst(s string) string {
    if s == "" {
        return s
    }
    return strings.ToLower(s[:1]) + s[1:]
}

func upperFirst(s string) string {
    if s == "" {
        return s
    }
    return strings.ToUpper(s[:1]) + s[1:]
}

func toCamel(s string) string {
    parts := strings.Split(s, "_")
    for i := 1; i < len(parts); i++ {
        parts[i] = upperFirst(parts[i])
    }
    return strings.Join(parts, "")
}

func toSnake(s string) string {
    var result []rune
    for i, r := range s {
        if i > 0 && r >= 'A' && r <= 'Z' {
            result = append(result, '_')
        }
        result = append(result, unicode.ToLower(r))
    }
    return string(result)
}

func toKebab(s string) string {
    return strings.ReplaceAll(toSnake(s), "_", "-")
}
```

需要在文件顶部添加 import：

```go
import (
    "bytes"
    "strings"
    "text/template"
    "unicode"
)
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd go-common && go test ./templateutil/... -v
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-common/templateutil/
git commit -m "feat(templateutil): add default function set (ToLower, ToUpper, ToCamel, etc.)"
```

---

### Task 1.3: 添加自定义函数注册测试

**Files:**
- Modify: `go-common/templateutil/templateutil_test.go`

**Interfaces:**
- Consumes: `Registry`, `Register()`
- Produces: 验证自定义函数可以注册并使用

- [ ] **Step 1: 写测试**

```go
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
```

- [ ] **Step 2: 运行测试验证通过**

```bash
cd go-common && go test ./templateutil/... -v
```

Expected: PASS（因为前面已经实现了 Register 和 RenderWith）

- [ ] **Step 3: 提交**

```bash
git add go-common/templateutil/
git commit -m "test(templateutil): add tests for custom function registration"
```

---

### Task 1.4: templateutil lint 和构建验证

**Files:**
- 无新文件

- [ ] **Step 1: 运行 lint**

```bash
cd go-common && golangci-lint run ./templateutil/...
```

Expected: 无错误

- [ ] **Step 2: 修复 lint 错误（如果有）**

根据 lint 输出修复问题，然后重新运行。

- [ ] **Step 3: 运行构建**

```bash
cd go-common && go build ./templateutil/...
```

Expected: 成功

- [ ] **Step 4: 运行测试**

```bash
cd go-common && go test ./templateutil/... -count=1 -v
```

Expected: PASS

- [ ] **Step 5: 提交（如果有修复）**

```bash
git add go-common/templateutil/
git commit -m "fix(templateutil): fix lint errors"
```

---

## 阶段 2：`executil`（无外部依赖，但逻辑较复杂）

### Task 2.1: 创建 executil 包骨架

**Files:**
- Create: `go-common/executil/executil.go`
- Create: `go-common/executil/executil_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `Runner`, `Cmd`, `Result`, `New()`

- [ ] **Step 1: 写失败的测试**

```go
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
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd go-common && go test ./executil/... -v
```

Expected: FAIL — `package github.com/byx-darwin/go-tools/go-common/executil is not found`

- [ ] **Step 3: 创建最小实现**

```go
// go-common/executil/executil.go
// Package executil 提供增强的命令执行包装器。
package executil

import (
    "bytes"
    "context"
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
        if exitErr, ok := err.(*exec.ExitError); ok {
            result.ExitCode = exitErr.ExitCode()
            result.Err = &ExitError{
                ExitCode: exitErr.ExitCode(),
                Stderr:   truncate(stderrBuf.Bytes(), 1024),
            }
        } else if ctx.Err() == context.DeadlineExceeded {
            result.Err = &TimeoutError{Duration: cmd.Timeout}
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

func truncate(b []byte, max int) []byte {
    if len(b) <= max {
        return b
    }
    return b[:max]
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd go-common && go test ./executil/... -v
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-common/executil/
git commit -m "feat(executil): add Runner, Cmd, Result types and basic Run"
```

---

### Task 2.2: 实现错误分类

**Files:**
- Modify: `go-common/executil/executil.go`
- Modify: `go-common/executil/executil_test.go`

**Interfaces:**
- Consumes: `Runner`, `Result`
- Produces: `NotFoundError`, `ExitError`, `TimeoutError`

- [ ] **Step 1: 写失败的测试**

```go
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
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd go-common && go test ./executil/... -v
```

Expected: FAIL — `NotFoundError`, `ExitError`, `TimeoutError` 未定义

- [ ] **Step 3: 实现错误类型**

```go
// 在 executil.go 添加
import "errors"

// NotFoundError 命令不存在。
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

// ExitError 命令退出码非零。
type ExitError struct {
    ExitCode int
    Stderr   []byte
}

func (e *ExitError) Error() string {
    return "command exited with code " + string(rune('0'+e.ExitCode))
}

// TimeoutError 命令执行超时。
type TimeoutError struct {
    Duration time.Duration
}

func (e *TimeoutError) Error() string {
    return "command timed out after " + e.Duration.String()
}

// 在 Run 方法中，修改错误处理部分：
if err != nil {
    if exitErr := new(exec.ExitError); errors.As(err, &exitErr) {
        result.ExitCode = exitErr.ExitCode()
        result.Err = &ExitError{
            ExitCode: exitErr.ExitCode(),
            Stderr:   truncate(stderrBuf.Bytes(), 1024),
        }
    } else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
        result.Err = &TimeoutError{Duration: cmd.Timeout}
    } else {
        result.Err = &NotFoundError{Name: cmd.Name}
    }
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd go-common && go test ./executil/... -v
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-common/executil/
git commit -m "feat(executil): add NotFoundError, ExitError, TimeoutError"
```

---

### Task 2.3: 实现流式输出和上下文取消

**Files:**
- Modify: `go-common/executil/executil_test.go`

**Interfaces:**
- Consumes: `Cmd.OnStdout`, `Cmd.OnStderr`, `context.Context`
- Produces: 实时输出回调、上下文取消支持

- [ ] **Step 1: 写测试**

```go
func TestRun_StreamingOutput(t *testing.T) {
    runner := executil.New()
    var lines []string
    result := runner.Run(context.Background(), &executil.Cmd{
        Name: "sh",
        Args: []string{"-c", "echo line1; echo line2"},
        OnStdout: func(line []byte) {
            lines = append(lines, string(bytes.TrimSpace(line)))
        },
    })
    require.NoError(t, result.Err)
    require.Contains(t, lines, "line1")
    require.Contains(t, lines, "line2")
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
```

- [ ] **Step 2: 运行测试验证通过**

```bash
cd go-common && go test ./executil/... -v
```

Expected: PASS（因为前面已经实现了 OnStdout 和 context 支持）

- [ ] **Step 3: 提交**

```bash
git add go-common/executil/
git commit -m "test(executil): add tests for streaming output and context cancellation"
```

---

### Task 2.4: executil lint 和构建验证

**Files:**
- 无新文件

- [ ] **Step 1: 运行 lint**

```bash
cd go-common && golangci-lint run ./executil/...
```

Expected: 无错误

- [ ] **Step 2: 修复 lint 错误（如果有）**

- [ ] **Step 3: 运行构建和测试**

```bash
cd go-common && go build ./executil/... && go test ./executil/... -count=1 -v
```

Expected: 成功

- [ ] **Step 4: 提交（如果有修复）**

```bash
git add go-common/executil/
git commit -m "fix(executil): fix lint errors"
```

---

## 阶段 3：`astutil/`（基于 dst，最复杂）

### Task 3.1: 添加 dave/dst 依赖

**Files:**
- Modify: `go-common/go.mod`

- [ ] **Step 1: 添加依赖**

```bash
cd go-common && go get github.com/dave/dst@latest
```

- [ ] **Step 2: 验证依赖**

```bash
cd go-common && go mod tidy
```

- [ ] **Step 3: 提交**

```bash
git add go-common/go.mod go-common/go.sum
git commit -m "deps(astutil): add github.com/dave/dst"
```

---

### Task 3.2: 创建 astutil 包骨架

**Files:**
- Create: `go-common/astutil/astutil.go`
- Create: `go-common/astutil/astutil_test.go`

**Interfaces:**
- Consumes: `github.com/dave/dst`
- Produces: `File`, `ParseFile()`, `ParseSource()`

- [ ] **Step 1: 写失败的测试**

```go
// go-common/astutil/astutil_test.go
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
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd go-common && go test ./astutil/... -v
```

Expected: FAIL — `package github.com/byx-darwin/go-tools/go-common/astutil is not found`

- [ ] **Step 3: 创建最小实现**

```go
// go-common/astutil/astutil.go
// Package astutil 提供通用 Go AST 操作库，基于 dave/dst 封装。
package astutil

import (
    "github.com/dave/dst"
    "github.com/dave/dst/decorator"
)

// File 表示一个可操作的 Go 源文件。
type File struct {
    file *dst.File
    dec  *decorator.Decorator
}

// ParseSource 从源代码解析。
func ParseSource(src []byte) (*File, error) {
    dec := decorator.NewDecorator(nil)
    f, err := dec.ParseFile("", src)
    if err != nil {
        return nil, err
    }
    return &File{file: f, dec: dec}, nil
}

// ParseFile 从文件路径解析。
func ParseFile(path string) (*File, error) {
    dec := decorator.NewDecorator(nil)
    f, err := dec.ParseFile(path, nil)
    if err != nil {
        return nil, err
    }
    return &File{file: f, dec: dec}, nil
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd go-common && go test ./astutil/... -v
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-common/astutil/
git commit -m "feat(astutil): add File, ParseFile, ParseSource"
```

---

### Task 3.3: 实现查询方法

**Files:**
- Create: `go-common/astutil/query.go`
- Modify: `go-common/astutil/astutil_test.go`

**Interfaces:**
- Consumes: `File`
- Produces: `FindFunctions()`, `FindFunction()`, `FindImports()`, `FindDecls()`

- [ ] **Step 1: 写失败的测试**

```go
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
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd go-common && go test ./astutil/... -v
```

Expected: FAIL — `FindFunctions`, `FindFunction`, `FindImports` 未定义

- [ ] **Step 3: 实现查询方法**

```go
// go-common/astutil/query.go
package astutil

import "github.com/dave/dst"

// FindFunctions 返回所有函数声明。
func (f *File) FindFunctions() []*dst.FuncDecl {
    var funcs []*dst.FuncDecl
    for _, decl := range f.file.Decls {
        if fn, ok := decl.(*dst.FuncDecl); ok {
            funcs = append(funcs, fn)
        }
    }
    return funcs
}

// FindFunction 按名称查找函数。
func (f *File) FindFunction(name string) *dst.FuncDecl {
    for _, fn := range f.FindFunctions() {
        if fn.Name.Name == name {
            return fn
        }
    }
    return nil
}

// FindImports 返回所有 import。
func (f *File) FindImports() []*dst.ImportSpec {
    var imports []*dst.ImportSpec
    for _, decl := range f.file.Decls {
        if genDecl, ok := decl.(*dst.GenDecl); ok {
            for _, spec := range genDecl.Specs {
                if imp, ok := spec.(*dst.ImportSpec); ok {
                    imports = append(imports, imp)
                }
            }
        }
    }
    return imports
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd go-common && go test ./astutil/... -v
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-common/astutil/
git commit -m "feat(astutil): add FindFunctions, FindFunction, FindImports"
```

---

### Task 3.4: 实现修改操作

**Files:**
- Create: `go-common/astutil/ops.go`
- Modify: `go-common/astutil/astutil_test.go`

**Interfaces:**
- Consumes: `File`, `dst.Node`
- Produces: `Apply()`, `AddImport()`, `RemoveImport()`, `InsertAfter()`, `ReplaceNode()`

- [ ] **Step 1: 写失败的测试**

```go
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
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd go-common && go test ./astutil/... -v
```

Expected: FAIL — `Apply`, `AddImport`, `RemoveImport` 未定义

- [ ] **Step 3: 实现操作**

```go
// go-common/astutil/ops.go
package astutil

import (
    "github.com/dave/dst"
    "github.com/dave/dst/dstutil"
)

// Op 表示一个 AST 修改操作。
type Op func(*dst.File)

// Apply 应用多个操作。
func (f *File) Apply(ops ...Op) {
    for _, op := range ops {
        op(f.file)
    }
}

// AddImport 添加 import（幂等）。
func AddImport(path string) Op {
    return func(f *dst.File) {
        // 检查是否已存在
        for _, imp := range f.Imports() {
            if imp.Path.Value == `"`+path+`"` {
                return
            }
        }
        // 添加
        dstutil.AddImport(f, path, nil)
    }
}

// RemoveImport 移除 import。
func RemoveImport(path string) Op {
    return func(f *dst.File) {
        dstutil.DeleteImport(f, path)
    }
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd go-common && go test ./astutil/... -v
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-common/astutil/
git commit -m "feat(astutil): add Apply, AddImport, RemoveImport"
```

---

### Task 3.5: 实现格式化输出

**Files:**
- Create: `go-common/astutil/format.go`
- Modify: `go-common/astutil/astutil_test.go`

**Interfaces:**
- Consumes: `File`
- Produces: `Format()`, `WriteTo()`

- [ ] **Step 1: 写失败的测试**

```go
func TestFile_Format(t *testing.T) {
    file, _ := astutil.ParseSource([]byte(`package main
import"fmt"
func main(){fmt.Println("hello")}
`))
    out, err := file.Format()
    require.NoError(t, err)
    require.Contains(t, out, "package main")
    require.Contains(t, out, `import "fmt"`)
}
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd go-common && go test ./astutil/... -v
```

Expected: FAIL — `Format` 未定义

- [ ] **Step 3: 实现格式化**

```go
// go-common/astutil/format.go
package astutil

import (
    "bytes"
    "os"

    "github.com/dave/dst/decorator"
)

// Format 返回格式化后的源码。
func (f *File) Format() ([]byte, error) {
    restorer := decorator.NewRestorer()
    var buf bytes.Buffer
    if err := restorer.Fprint(&buf, f.file); err != nil {
        return nil, err
    }
    return buf.Bytes(), nil
}

// WriteTo 写入文件。
func (f *File) WriteTo(path string) error {
    out, err := f.Format()
    if err != nil {
        return err
    }
    return os.WriteFile(path, out, 0644)
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd go-common && go test ./astutil/... -v
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-common/astutil/
git commit -m "feat(astutil): add Format, WriteTo"
```

---

### Task 3.6: astutil lint 和构建验证

**Files:**
- 无新文件

- [ ] **Step 1: 运行 lint**

```bash
cd go-common && golangci-lint run ./astutil/...
```

Expected: 无错误

- [ ] **Step 2: 修复 lint 错误（如果有）**

- [ ] **Step 3: 运行构建和测试**

```bash
cd go-common && go build ./astutil/... && go test ./astutil/... -count=1 -v
```

Expected: 成功

- [ ] **Step 4: 提交（如果有修复）**

```bash
git add go-common/astutil/
git commit -m "fix(astutil): fix lint errors"
```

---

## 阶段 4：`astutil/gen/`（jennifer 风格代码生成）

### Task 4.1: 创建 gen 包骨架

**Files:**
- Create: `go-common/astutil/gen/gen.go`
- Create: `go-common/astutil/gen/gen_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `File`, `NewFile()`, `Render()`

- [ ] **Step 1: 写失败的测试**

```go
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
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd go-common && go test ./astutil/gen/... -v
```

Expected: FAIL — `package github.com/byx-darwin/go-tools/go-common/astutil/gen is not found`

- [ ] **Step 3: 创建最小实现**

```go
// go-common/astutil/gen/gen.go
// Package gen 提供 jennifer 风格的流式 API，用于从零生成 Go 代码。
package gen

import (
    "bytes"
    "fmt"
)

// File 表示一个 Go 源文件。
type File struct {
    pkg     string
    imports []string
    decls   []string
}

// NewFile 创建新文件。
func NewFile(pkg string) *File {
    return &File{pkg: pkg}
}

// Import 添加 import。
func (f *File) Import(path string) *File {
    f.imports = append(f.imports, path)
    return f
}

// Render 返回格式化的源码。
func (f *File) Render() (string, error) {
    var buf bytes.Buffer
    fmt.Fprintf(&buf, "package %s\n\n", f.pkg)
    if len(f.imports) > 0 {
        buf.WriteString("import (\n")
        for _, imp := range f.imports {
            fmt.Fprintf(&buf, "\t%q\n", imp)
        }
        buf.WriteString(")\n\n")
    }
    for _, decl := range f.decls {
        buf.WriteString(decl)
        buf.WriteString("\n\n")
    }
    return buf.String(), nil
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd go-common && go test ./astutil/gen/... -v
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-common/astutil/gen/
git commit -m "feat(astutil/gen): add File, NewFile, Import, Render"
```

---

### Task 4.2: 实现函数生成

**Files:**
- Create: `go-common/astutil/gen/stmt.go`
- Modify: `go-common/astutil/gen/gen.go`
- Modify: `go-common/astutil/gen/gen_test.go`

**Interfaces:**
- Consumes: `File`
- Produces: `Func()`, `Params()`, `Results()`, `Body()`

- [ ] **Step 1: 写失败的测试**

```go
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
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd go-common && go test ./astutil/gen/... -v
```

Expected: FAIL — `Func`, `Param` 未定义

- [ ] **Step 3: 实现函数生成**

```go
// go-common/astutil/gen/stmt.go
package gen

import "fmt"

// Param 创建参数。
func Param(name, typ string) string {
    return fmt.Sprintf("%s %s", name, typ)
}

// FuncDecl 函数声明构建器。
type FuncDecl struct {
    file    *File
    name    string
    params  []string
    results []string
    body    []string
}

// Func 添加函数。
func (f *File) Func(name string) *FuncDecl {
    fd := &FuncDecl{file: f, name: name}
    f.decls = append(f.decls, "") // 占位
    return fd
}

// Params 设置参数。
func (fd *FuncDecl) Params(params ...string) *FuncDecl {
    fd.params = params
    return fd
}

// Results 设置返回值。
func (fd *FuncDecl) Results(results ...string) *FuncDecl {
    fd.results = results
    return fd
}

// Body 设置函数体。
func (fd *FuncDecl) Body(stmts ...string) *FuncDecl {
    fd.body = stmts
    fd.file.decls[len(fd.file.decls)-1] = fd.render()
    return fd
}

func (fd *FuncDecl) render() string {
    sig := fmt.Sprintf("func %s(", fd.name)
    for i, p := range fd.params {
        if i > 0 {
            sig += ", "
        }
        sig += p
    }
    sig += ")"
    if len(fd.results) > 0 {
        sig += " ("
        for i, r := range fd.results {
            if i > 0 {
                sig += ", "
            }
            sig += r
        }
        sig += ")"
    }
    body := ""
    if len(fd.body) > 0 {
        body = "\n"
        for _, stmt := range fd.body {
            body += "\t" + stmt + "\n"
        }
        body += "}"
    } else {
        body = " {}"
    }
    return sig + " " + body
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd go-common && go test ./astutil/gen/... -v
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-common/astutil/gen/
git commit -m "feat(astutil/gen): add Func, Params, Results, Body"
```

---

### Task 4.3: 实现结构体生成

**Files:**
- Modify: `go-common/astutil/gen/gen.go`
- Modify: `go-common/astutil/gen/gen_test.go`

**Interfaces:**
- Consumes: `File`
- Produces: `Struct()`, `Field()`, `Tag()`

- [ ] **Step 1: 写失败的测试**

```go
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
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd go-common && go test ./astutil/gen/... -v
```

Expected: FAIL — `Struct`, `Field`, `Tag` 未定义

- [ ] **Step 3: 实现结构体生成**

```go
// 在 gen.go 添加
// Struct 添加结构体。
func (f *File) Struct(name string) *StructDecl {
    sd := &StructDecl{file: f, name: name}
    f.decls = append(f.decls, "") // 占位
    return sd
}

// StructDecl 结构体声明构建器。
type StructDecl struct {
    file   *File
    name   string
    fields []structField
}

type structField struct {
    name string
    typ  string
    tag  string
}

// Field 添加字段。
func (sd *StructDecl) Field(name, typ string) *StructDecl {
    sd.fields = append(sd.fields, structField{name: name, typ: typ})
    sd.file.decls[len(sd.file.decls)-1] = sd.render()
    return sd
}

// Tag 为最后一个字段添加 tag。
func (sd *StructDecl) Tag(tag string) *StructDecl {
    if len(sd.fields) > 0 {
        sd.fields[len(sd.fields)-1].tag = tag
        sd.file.decls[len(sd.file.decls)-1] = sd.render()
    }
    return sd
}

func (sd *StructDecl) render() string {
    result := fmt.Sprintf("type %s struct {\n", sd.name)
    for _, f := range sd.fields {
        line := fmt.Sprintf("\t%s %s", f.name, f.typ)
        if f.tag != "" {
            line += " `" + f.tag + "`"
        }
        result += line + "\n"
    }
    result += "}"
    return result
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd go-common && go test ./astutil/gen/... -v
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-common/astutil/gen/
git commit -m "feat(astutil/gen): add Struct, Field, Tag"
```

---

### Task 4.4: gen lint 和构建验证

**Files:**
- 无新文件

- [ ] **Step 1: 运行 lint**

```bash
cd go-common && golangci-lint run ./astutil/gen/...
```

Expected: 无错误

- [ ] **Step 2: 修复 lint 错误（如果有）**

- [ ] **Step 3: 运行构建和测试**

```bash
cd go-common && go build ./astutil/gen/... && go test ./astutil/gen/... -count=1 -v
```

Expected: 成功

- [ ] **Step 4: 提交（如果有修复）**

```bash
git add go-common/astutil/gen/
git commit -m "fix(astutil/gen): fix lint errors"
```

---

## 阶段 5：整体验证

### Task 5.1: 全模块构建和测试

**Files:**
- 无新文件

- [ ] **Step 1: 构建所有新包**

```bash
cd go-common && go build ./templateutil/... ./executil/... ./astutil/... ./astutil/gen/...
```

Expected: 成功

- [ ] **Step 2: 测试所有新包**

```bash
cd go-common && go test ./templateutil/... ./executil/... ./astutil/... ./astutil/gen/... -count=1 -v
```

Expected: PASS

- [ ] **Step 3: 运行 lint**

```bash
cd go-common && golangci-lint run ./templateutil/... ./executil/... ./astutil/... ./astutil/gen/...
```

Expected: 无错误

- [ ] **Step 4: 运行全模块测试**

```bash
go test ./go-common/... -count=1
```

Expected: PASS

- [ ] **Step 5: 提交（如果有修复）**

```bash
git add .
git commit -m "test: all new packages pass build, test, and lint"
```

---

### Task 5.2: 更新文档

**Files:**
- Modify: `README.md`（如果存在）

- [ ] **Step 1: 添加新包说明**

在 README 中添加新包的说明（如果 README 存在）。

- [ ] **Step 2: 提交**

```bash
git add README.md
git commit -m "docs: add templateutil, executil, astutil to README"
```

---

## 完成标准检查

- [ ] 4 个包全部实现：`templateutil`, `executil`, `astutil`, `astutil/gen`
- [ ] 所有包通过单元测试
- [ ] 所有包通过 lint
- [ ] 所有导出符号有 godoc 注释
- [ ] 代码符合 `.claude/rules/go.md` 规范

---

## 执行选择

**Plan complete and saved to `docs/superpowers/plans/2026-06-25-tooling-extraction.md`. Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
