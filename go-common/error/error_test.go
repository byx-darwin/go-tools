package error

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// init 注册测试专用码 39911 → 404。
// 放在 init() 而非某个 Test 函数内，保证与测试文件/函数的执行顺序无关
// （Go 按文件名字母序执行：error_test.go 先于 httpstatus_test.go）。
func init() {
	RegisterHTTPStatuses(map[int]int{39911: 404})
}

// ── 错误码范围 ──

func TestCodeConstants(t *testing.T) {
	assert.Less(t, FrameworkCodeMax, MiddlewareCodeMin)
	assert.Less(t, MiddlewareCodeMax, AuthCodeMin)
	assert.Less(t, AuthCodeMax, ProjectCodeMin)
}

// auth 段（40000-40099）是业务码段下限，行为须与迁移前逐值一致。
func TestAuthBandBoundary(t *testing.T) {
	assert.Equal(t, 40000, AuthCodeMin)
	assert.Equal(t, 40099, AuthCodeMax)
	assert.Equal(t, 40100, ProjectCodeMin)

	// 业务码判定：auth 段 + project 段均为业务码
	assert.True(t, IsBusinessErrorCode(AuthCodeMin))    // 40000
	assert.True(t, IsBusinessErrorCode(AuthCodeMax))    // 40099
	assert.True(t, IsBusinessErrorCode(ProjectCodeMin)) // 40100

	// HTTP 兜底：auth 段未注册码 → 200
	assert.Equal(t, 200, HTTPStatus(Code(40050).Public("auth_band").Wrap(errors.New("x"))))
}

// ── Code / Extract ──

func TestCode_Basic(t *testing.T) {
	original := errors.New("original error")
	err := Code(12345).Public("custom_error").Wrap(original)

	code, public := Extract(err)
	assert.Equal(t, 12345, code)
	assert.Equal(t, "custom_error", public)
}

func TestIn_Basic(t *testing.T) {
	original := errors.New("auth failed")
	err := In("auth").Code(12346).Public("token_expired").Wrap(original)

	code, public := Extract(err)
	assert.Equal(t, 12346, code)
	assert.Equal(t, "token_expired", public)
}

func TestExtract_NilError(t *testing.T) {
	code, public := Extract(nil)
	assert.Equal(t, 0, code)
	assert.Empty(t, public)
}

func TestExtract_NonOopsError(t *testing.T) {
	err := errors.New("plain error")
	code, public := Extract(err)
	assert.Equal(t, 0, code)
	assert.Empty(t, public)
}

func TestExtractWithFallback_NonOops(t *testing.T) {
	err := errors.New("plain error")
	code, public := ExtractWithFallback(err, 99999)
	assert.Equal(t, 99999, code)
	assert.Equal(t, "plain error", public)
}

func TestExtractWithFallback_OopsError(t *testing.T) {
	err := Code(12345).Public("custom").Wrap(errors.New("inner"))
	code, public := ExtractWithFallback(err, 99999)
	assert.Equal(t, 12345, code)
	assert.Equal(t, "custom", public)
}

func TestAsOopsError(t *testing.T) {
	err := Code(10001).Public("test").Wrap(errors.New("inner"))

	oopsErr, ok := AsOopsError(err)
	assert.True(t, ok)
	assert.Equal(t, 10001, oopsErr.Code())
}

func TestAsOopsError_NonOops(t *testing.T) {
	err := errors.New("plain")

	_, ok := AsOopsError(err)
	assert.False(t, ok)
}

// ── HTTP 状态码映射（范围兜底；细粒度映射见各属主模块测试）──

func TestHTTPStatus_Fallback(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"business code → 200", Code(40001).Public("data_duplicate").Wrap(errors.New("dup")), 200},
		{"unregistered infra code → 500", Code(20999).Public("unregistered").Wrap(errors.New("x")), 500},
		{"plain error → 200", errors.New("plain"), 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, HTTPStatus(tt.err))
		})
	}
}

func TestIsClientError(t *testing.T) {
	// 39911 已由本文件 init() 注册为 404。
	assert.True(t, IsClientError(39911))
	assert.False(t, IsClientError(40001)) // 业务码 → 200
	assert.False(t, IsClientError(20999)) // 未注册 → 500
}

func TestIsServerError(t *testing.T) {
	assert.True(t, IsServerError(20999))  // 未注册 >0 → 500
	assert.False(t, IsServerError(40001)) // 业务码 → 200
	assert.False(t, IsServerError(0))     // 无码 → 200
}

func TestIsBusinessErrorCode(t *testing.T) {
	assert.True(t, IsBusinessErrorCode(40010))
	assert.True(t, IsBusinessErrorCode(40001))
	assert.False(t, IsBusinessErrorCode(10000))
	assert.False(t, IsBusinessErrorCode(20001))
}
