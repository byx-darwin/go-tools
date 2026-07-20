package log

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/samber/oops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDomainLogger_Decision_Accepted(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := &Logger{Logger: slog.New(handler), level: slog.LevelDebug}
	SetDefault(logger)
	defer SetDefault(nil)

	dl := NewDomainLogger("order")
	dl.Decision("订单创建", true, "order_id", "123")

	var result map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Equal(t, "INFO", result["level"])
	assert.Equal(t, "order", result["domain"])
	assert.Equal(t, "decision", result["log_type"])
	assert.Equal(t, true, result["accepted"])
	assert.Equal(t, "订单创建", result["msg"])
}

func TestDomainLogger_Decision_Rejected(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := &Logger{Logger: slog.New(handler), level: slog.LevelDebug}
	SetDefault(logger)
	defer SetDefault(nil)

	dl := NewDomainLogger("order")
	dl.Decision("余额不足拒绝", false, "user_id", "456")

	var result map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Equal(t, "WARN", result["level"])
	assert.Equal(t, "order", result["domain"])
	assert.Equal(t, "decision", result["log_type"])
	assert.Equal(t, false, result["accepted"])
}

func TestDomainLogger_Event(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := &Logger{Logger: slog.New(handler), level: slog.LevelDebug}
	SetDefault(logger)
	defer SetDefault(nil)

	dl := NewDomainLogger("order")
	dl.Event("订单已创建", "order_id", "789")

	var result map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Equal(t, "INFO", result["level"])
	assert.Equal(t, "order", result["domain"])
	assert.Equal(t, "event", result["log_type"])
}

func TestDomainLogger_Error_WithRegularError(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := &Logger{Logger: slog.New(handler), level: slog.LevelDebug}
	SetDefault(logger)
	defer SetDefault(nil)

	dl := NewDomainLogger("payment")
	regularErr := errors.New("timeout") // 普通错误，非 oops
	dl.Error("支付处理失败", regularErr)

	var result map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Equal(t, "ERROR", result["level"])
	assert.Equal(t, "payment", result["domain"])
	assert.Equal(t, "error", result["log_type"])
	assert.Equal(t, "timeout", result["error"])
}

func TestDomainLogger_Error_WithOopsError(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := &Logger{Logger: slog.New(handler), level: slog.LevelDebug}
	SetDefault(logger)
	defer SetDefault(nil)

	dl := NewDomainLogger("payment")
	oopsErr := oops.Code("TEST_ERROR").In("test").Wrap(errors.New("something"))
	dl.Error("oops 错误测试", oopsErr)

	var result map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Equal(t, "ERROR", result["level"])
	assert.Equal(t, "payment", result["domain"])
	assert.Equal(t, "error", result["log_type"])
	assert.Equal(t, "something", result["error"])
	assert.Equal(t, "TEST_ERROR", result["error.code"])
	assert.Equal(t, "test", result["error.domain"])
}

func TestDomainLogger_Error_WithNilError(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := &Logger{Logger: slog.New(handler), level: slog.LevelDebug}
	SetDefault(logger)
	defer SetDefault(nil)

	dl := NewDomainLogger("order")
	dl.Error("异常日志", nil) // nil error 不应 panic

	var result map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Equal(t, "ERROR", result["level"])
	assert.Equal(t, "order", result["domain"])
}

func TestDomainLogger_EmptyDomain(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := &Logger{Logger: slog.New(handler), level: slog.LevelDebug}
	SetDefault(logger)
	defer SetDefault(nil)

	dl := NewDomainLogger("") // 空 domain 不应 panic
	dl.Event("测试事件")

	var result map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Equal(t, "event", result["log_type"])
	// domain 字段可能不存在或为空
}

func TestDomainLogger_Concurrent(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := &Logger{Logger: slog.New(handler), level: slog.LevelDebug}
	SetDefault(logger)
	defer SetDefault(nil)

	dl := NewDomainLogger("order")
	done := make(chan struct{})

	// 并发写入验证无竞态
	for i := 0; i < 10; i++ {
		go func() {
			dl.Decision("测试决策", true, "key", "val")
			dl.Event("测试事件", "key", "val")
			dl.Error("测试错误", errors.New("test err"), "key", "val")
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
