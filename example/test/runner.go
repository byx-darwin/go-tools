// Package main 提供 example 项目的端到端测试运行器。
//
// 用法：
//
//	go run ./test/ -mode local            # 内存存储，跳过中间件测试
//	go run ./test/ -mode docker           # 真实中间件服务
//	go run ./test/ -mode local -report    # 生成 test/report.md
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// ── 类型定义 ──

// TestResult 单个测试用例的执行结果。
type TestResult struct {
	// Name 测试用例名称。
	Name string

	// Status 执行状态：pass / fail / skip。
	Status string

	// Duration 执行耗时。
	Duration time.Duration

	// Error 失败时的错误信息。
	Error string

	// Expected 期望值（失败时用于对比）。
	Expected string

	// Actual 实际值（失败时用于对比）。
	Actual string

	// Category 测试分类（go-common / go-auth / middleware / config / rpc）。
	Category string
}

// TestCase 单个测试用例定义。
type TestCase struct {
	// Name 测试用例名称（唯一）。
	Name string

	// Method HTTP 方法（GET / POST / DELETE）。
	Method string

	// Path 请求路径（如 /health）。
	Path string

	// Body JSON 请求体（POST 时使用）。
	Body string

	// Headers 额外请求头。
	Headers map[string]string

	// Assert 断言函数。
	Assert Assertion

	// SkipIf 跳过条件（nil 表示始终执行）。
	SkipIf SkipCondition

	// DependsOn 前置依赖测试名称（链式执行）。
	DependsOn string

	// Category 测试分类。
	Category string

	// AfterRun 执行后的钩子（用于将结果存入 testContext）。
	// 接收响应体和状态码。
	AfterRun func(statusCode int, body []byte) error
}

// Assertion 断言函数签名。
type Assertion func(statusCode int, body []byte) error

// SkipCondition 跳过条件函数签名。
//
// 返回 skip=true 时测试跳过，reason 为跳过原因。
type SkipCondition func(mode string) (skip bool, reason string)

// ── 全局状态 ──

// testContext 存储测试间共享的数据（如 JWT token）。
var testContext = map[string]string{}

// baseURL 测试目标地址。
var baseURL = "http://localhost:8080"

// ── main ──

func main() {
	mode := flag.String("mode", "local", "test mode: local or docker")
	report := flag.Bool("report", false, "generate markdown report at test/report.md")
	dir := flag.String("dir", "", "example project directory (default: auto-detect via runtime.Caller)")
	flag.Parse()
	_ = dir // 在 exampleDir() 中通过 flag.Lookup 访问

	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║   go-tools example test runner           ║")
	fmt.Printf("║   mode: %-8s  report: %-5v                ║\n", *mode, *report)
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()

	// 1. 启动 example 服务器。
	fmt.Print("▶ starting example server... ")
	cmd, err := startServer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n✗ failed to start server: %v\n", err)
		os.Exit(1)
	}
	defer stopServer(cmd)

	// 2. 等待 /health 就绪。
	if err := waitForHealth(30 * time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "\n✗ server not ready: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("ready")
	fmt.Println()

	// 3. 运行测试。
	cases := allCases()
	results := runCases(cases, *mode)

	// 4. 输出结果。
	printTerminalReport(results)

	if *report {
		if err := writeMarkdownReport(results); err != nil {
			fmt.Fprintf(os.Stderr, "✗ write report: %v\n", err)
		} else {
			fmt.Println("✓ report written to test/report.md")
		}
	}

	// 5. 根据失败数退出。
	failCount := countByStatus(results, "fail")
	if failCount > 0 {
		os.Exit(1)
	}
}

// ── 服务器管理 ──

// startServer 启动 example 服务器子进程。
func startServer() (*exec.Cmd, error) {
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = exampleDir()
	cmd.Stdout = os.Stderr // 服务器日志输出到 stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}
	return cmd, nil
}

// stopServer 停止 example 服务器子进程。
func stopServer(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	fmt.Print("\n▶ shutting down server... ")
	_ = cmd.Process.Signal(syscall.SIGTERM)

	// 等待进程退出（最多 5 秒）。
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		fmt.Println("done")
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		fmt.Println("killed")
	}
}

// waitForHealth 轮询 /health 直到成功或超时。
func waitForHealth(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout after %v", timeout)
}

// exampleDir 返回 example/ 目录的绝对路径。
//
// 优先级：-dir flag > EXAMPLE_DIR 环境变量 > runtime.Caller 推导。
func exampleDir() string {
	// 1. -dir flag（在 main 中解析）。
	if dirFlag := flag.Lookup("dir"); dirFlag != nil {
		if d := dirFlag.Value.String(); d != "" {
			return d
		}
	}
	// 2. EXAMPLE_DIR 环境变量。
	if d := os.Getenv("EXAMPLE_DIR"); d != "" {
		return d
	}
	// 3. 通过 runtime.Caller 推导（test/runner.go 位于 example/test/，上级即 example/）。
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		// 获取 test/ 所在目录。
		dir := filename
		for i := len(dir) - 1; i >= 0; i-- {
			if dir[i] == '/' || dir[i] == '\\' {
				dir = dir[:i]
				break
			}
		}
		// 再上一级即 example/。
		for i := len(dir) - 1; i >= 0; i-- {
			if dir[i] == '/' || dir[i] == '\\' {
				return dir[:i+1]
			}
		}
	}
	// Fallback to relative path.
	return ".."
}

// ── 测试执行 ──

// runCases 按顺序执行所有测试用例。
func runCases(cases []TestCase, mode string) []TestResult {
	var results []TestResult
	skipped := map[string]bool{}

	for _, tc := range cases {
		// 检查依赖是否跳过/失败。
		if tc.DependsOn != "" && skipped[tc.DependsOn] {
			skipped[tc.Name] = true
			results = append(results, TestResult{
				Name:     tc.Name,
				Status:   "skip",
				Category: tc.Category,
				Error:    "dependency skipped: " + tc.DependsOn,
			})
			continue
		}

		// 检查 SkipIf 条件。
		if tc.SkipIf != nil {
			if skip, reason := tc.SkipIf(mode); skip {
				skipped[tc.Name] = true
				results = append(results, TestResult{
					Name:     tc.Name,
					Status:   "skip",
					Category: tc.Category,
					Error:    reason,
				})
				continue
			}
		}

		// 执行测试。
		start := time.Now()
		result := executeCase(tc)
		result.Duration = time.Since(start)
		result.Category = tc.Category

		// 记录依赖状态（失败时后续依赖也跳过）。
		if result.Status != "pass" {
			skipped[tc.Name] = true
		}

		results = append(results, result)
	}

	return results
}

// executeCase 执行单个测试用例。
func executeCase(tc TestCase) TestResult {
	result := TestResult{Name: tc.Name}

	statusCode, body, err := doRequest(tc)
	if err != nil {
		result.Status = "fail"
		result.Error = err.Error()
		return result
	}

	if tc.Assert != nil {
		if err := tc.Assert(statusCode, body); err != nil {
			result.Status = "fail"
			result.Error = err.Error()
			return result
		}
	}

	// 执行 AfterRun 钩子（存储上下文数据）。
	if tc.AfterRun != nil {
		if err := tc.AfterRun(statusCode, body); err != nil {
			result.Status = "fail"
			result.Error = "after-run hook: " + err.Error()
			return result
		}
	}

	result.Status = "pass"
	return result
}

// expandVars 将 s 中的 ${key} 替换为 testContext 中的值。
func expandVars(s string) string {
	if !strings.Contains(s, "${") {
		return s
	}
	result := s
	for k, v := range testContext {
		result = strings.ReplaceAll(result, "${"+k+"}", v)
	}
	return result
}

// doRequest 发送 HTTP 请求并返回状态码、响应体。
func doRequest(tc TestCase) (int, []byte, error) {
	path := expandVars(tc.Path)
	body := expandVars(tc.Body)

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(context.Background(), tc.Method, baseURL+path, bodyReader)
	if err != nil {
		return 0, nil, fmt.Errorf("create request: %w", err)
	}

	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range tc.Headers {
		req.Header.Set(k, expandVars(v))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read body: %w", err)
	}

	return resp.StatusCode, respBody, nil
}

// ── 断言辅助 ──

// statusCode 断言 HTTP 状态码。
func statusCode(code int) Assertion {
	return func(got int, _ []byte) error {
		if got != code {
			return fmt.Errorf("status code: want %d, got %d", code, got)
		}
		return nil
	}
}

// jsonHas 断言 JSON 响应体包含指定顶层字段。
func jsonHas(fields ...string) Assertion {
	return func(_ int, body []byte) error {
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			return fmt.Errorf("json parse: %w", err)
		}
		for _, f := range fields {
			if _, ok := m[f]; !ok {
				return fmt.Errorf("missing field %q", f)
			}
		}
		return nil
	}
}

// and 组合多个断言。
func and(assertions ...Assertion) Assertion {
	return func(sc int, body []byte) error {
		for _, a := range assertions {
			if err := a(sc, body); err != nil {
				return err
			}
		}
		return nil
	}
}

// dataFieldHas 断言 JSON 响应 data 字段包含指定子字段。
func dataFieldHas(fields ...string) Assertion {
	return func(_ int, body []byte) error {
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			return fmt.Errorf("json parse: %w", err)
		}
		data, ok := m["data"].(map[string]any)
		if !ok {
			return fmt.Errorf("missing or invalid \"data\" field")
		}
		for _, f := range fields {
			if _, ok := data[f]; !ok {
				return fmt.Errorf("data missing field %q", f)
			}
		}
		return nil
	}
}

// ── 跳过条件 ──

// serviceNotAvailable 当服务不可用时跳过测试（local 模式）。
func serviceNotAvailable(service string) SkipCondition {
	return func(mode string) (bool, string) {
		if mode == "local" {
			return true, service + " not available in local mode"
		}
		return false, ""
	}
}

// ── 工具函数 ──

// parseJSON 解析 JSON 响应体为 map。
func parseJSON(body []byte) (map[string]any, error) {
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// countByStatus 统计指定状态的测试数量。
func countByStatus(results []TestResult, status string) int {
	n := 0
	for _, r := range results {
		if r.Status == status {
			n++
		}
	}
	return n
}

// extractToken 从 JSON data 中提取指定字段存入 testContext。
func extractToken(body []byte, field, contextKey string) error {
	m, err := parseJSON(body)
	if err != nil {
		return err
	}
	data, ok := m["data"].(map[string]any)
	if !ok {
		return fmt.Errorf("missing data field")
	}
	val, ok := data[field].(string)
	if !ok {
		return fmt.Errorf("missing field %q in data", field)
	}
	testContext[contextKey] = val
	return nil
}
