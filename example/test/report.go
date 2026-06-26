package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// ── ANSI 颜色码 ──

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

// ── 终端报告 ──

// printTerminalReport 输出彩色终端测试报告。
func printTerminalReport(results []TestResult) {
	total := len(results)
	passed := countByStatus(results, "pass")
	failed := countByStatus(results, "fail")
	skipped := countByStatus(results, "skip")

	totalDuration := time.Duration(0)
	for _, r := range results {
		totalDuration += r.Duration
	}

	fmt.Printf("%s%s━━━ Test Results ━━━%s\n", colorBold, colorCyan, colorReset)
	fmt.Println()

	// 按分类输出。
	categories := []string{"health", "go-common", "go-auth", "auth-protected", "middleware", "config", "rpc"}
	for _, cat := range categories {
		catResults := filterByCategory(results, cat)
		if len(catResults) == 0 {
			continue
		}
		fmt.Printf("  %s%s%s\n", colorCyan, cat, colorReset)
		for _, r := range catResults {
			printOneResult(r)
		}
		fmt.Println()
	}

	// 未分类的。
	other := filterByCategory(results, "")
	for _, cat := range categories {
		// 跳过已输出的分类。
		_ = cat
	}
	if len(other) > 0 {
		fmt.Printf("  %sother%s\n", colorCyan, colorReset)
		for _, r := range other {
			printOneResult(r)
		}
		fmt.Println()
	}

	// 汇总。
	fmt.Printf("%s%s━━━ Summary ━━━%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("  Total:   %d\n", total)
	fmt.Printf("  %sPassed:  %d%s\n", colorGreen, passed, colorReset)
	if failed > 0 {
		fmt.Printf("  %sFailed:  %d%s\n", colorRed, failed, colorReset)
	} else {
		fmt.Printf("  Failed:  0\n")
	}
	if skipped > 0 {
		fmt.Printf("  %sSkipped: %d%s\n", colorYellow, skipped, colorReset)
	} else {
		fmt.Printf("  Skipped: 0\n")
	}
	fmt.Printf("  Duration: %v\n", totalDuration.Round(time.Millisecond))
	fmt.Println()

	if failed > 0 {
		fmt.Printf("%s%s━━━ Failure Details ━━━%s\n", colorBold, colorRed, colorReset)
		fmt.Println()
		for _, r := range results {
			if r.Status != "fail" {
				continue
			}
			fmt.Printf("  %s✗ %s%s\n", colorRed, r.Name, colorReset)
			fmt.Printf("    %s%s%s\n", colorRed, r.Error, colorReset)
			if r.Expected != "" || r.Actual != "" {
				fmt.Printf("    expected: %s\n", r.Expected)
				fmt.Printf("    actual:   %s\n", r.Actual)
			}
			fmt.Println()
		}
	}
}

// printOneResult 输出单个测试结果行。
func printOneResult(r TestResult) {
	switch r.Status {
	case "pass":
		fmt.Printf("    %s✓%s %s %s(%v)%s\n", colorGreen, colorReset, r.Name, colorGray, r.Duration.Round(time.Millisecond), colorReset)
	case "fail":
		fmt.Printf("    %s✗%s %s %s(%v)%s\n", colorRed, colorReset, r.Name, colorGray, r.Duration.Round(time.Millisecond), colorReset)
		fmt.Printf("      %s↳ %s%s\n", colorRed, r.Error, colorReset)
	case "skip":
		fmt.Printf("    %s◦%s %s %s(skip: %s)%s\n", colorYellow, colorReset, r.Name, colorGray, r.Error, colorReset)
	}
}

// filterByCategory 按分类过滤结果。
func filterByCategory(results []TestResult, cat string) []TestResult {
	var out []TestResult
	for _, r := range results {
		if r.Category == cat {
			out = append(out, r)
		}
	}
	return out
}

// ── Markdown 报告 ──

// writeMarkdownReport 生成 Markdown 格式测试报告。
func writeMarkdownReport(results []TestResult) error {
	total := len(results)
	passed := countByStatus(results, "pass")
	failed := countByStatus(results, "fail")
	skipped := countByStatus(results, "skip")

	totalDuration := time.Duration(0)
	for _, r := range results {
		totalDuration += r.Duration
	}

	var b strings.Builder

	b.WriteString("# Test Report\n\n")
	b.WriteString(fmt.Sprintf("**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// 汇总表。
	b.WriteString("## Summary\n\n")
	b.WriteString("| Metric | Value |\n")
	b.WriteString("|--------|-------|\n")
	b.WriteString(fmt.Sprintf("| Total | %d |\n", total))
	b.WriteString(fmt.Sprintf("| ✅ Passed | %d |\n", passed))
	b.WriteString(fmt.Sprintf("| ❌ Failed | %d |\n", failed))
	b.WriteString(fmt.Sprintf("| ⏭ Skipped | %d |\n", skipped))
	b.WriteString(fmt.Sprintf("| Duration | %v |\n\n", totalDuration.Round(time.Millisecond)))

	// 按分类列出详情。
	categories := []struct {
		key  string
		name string
	}{
		{"health", "Health"},
		{"go-common", "go-common"},
		{"go-auth", "go-auth"},
		{"auth-protected", "Auth Protected Routes"},
		{"middleware", "Middleware"},
		{"config", "Config"},
		{"rpc", "RPC"},
		{"", "Other"},
	}

	b.WriteString("## Results by Category\n\n")
	for _, cat := range categories {
		catResults := filterByCategory(results, cat.key)
		if len(catResults) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("### %s\n\n", cat.name))
		b.WriteString("| Status | Test | Duration | Details |\n")
		b.WriteString("|--------|------|----------|--------|\n")
		for _, r := range catResults {
			status := statusIcon(r.Status)
			detail := ""
			if r.Error != "" {
				detail = escapeMarkdown(r.Error)
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %v | %s |\n",
				status, r.Name, r.Duration.Round(time.Millisecond), detail))
		}
		b.WriteString("\n")
	}

	// 失败详情。
	if failed > 0 {
		b.WriteString("## Failure Details\n\n")
		for _, r := range results {
			if r.Status != "fail" {
				continue
			}
			b.WriteString(fmt.Sprintf("### ❌ %s\n\n", r.Name))
			b.WriteString(fmt.Sprintf("**Error:** %s\n\n", r.Error))
			if r.Expected != "" {
				b.WriteString(fmt.Sprintf("**Expected:** `%s`\n\n", r.Expected))
			}
			if r.Actual != "" {
				b.WriteString(fmt.Sprintf("**Actual:** `%s`\n\n", r.Actual))
			}
		}
	}

	return os.WriteFile("test/report.md", []byte(b.String()), 0o644) //nolint:gosec // test report file
}

// statusIcon 返回状态对应的 Markdown 图标。
func statusIcon(status string) string {
	switch status {
	case "pass":
		return "✅"
	case "fail":
		return "❌"
	case "skip":
		return "⏭"
	default:
		return "?"
	}
}

// escapeMarkdown 转义 Markdown 表格中的特殊字符。
func escapeMarkdown(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
