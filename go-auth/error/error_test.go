package autherror

import (
	"errors"
	"testing"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/stretchr/testify/assert"
)

// TestCodeConstants 验证错误码在 40000-40099 范围内。
func TestCodeConstants(t *testing.T) {
	tests := []struct {
		name string
		code int
	}{
		{"CodeTokenInvalid", CodeTokenInvalid},
		{"CodeTokenExpired", CodeTokenExpired},
		{"CodeTokenRevoked", CodeTokenRevoked},
		{"CodeDeviceKicked", CodeDeviceKicked},
		{"CodeSessionInvalid", CodeSessionInvalid},
		{"CodeSessionExpired", CodeSessionExpired},
		{"CodeJWTSignFailed", CodeJWTSignFailed},
		{"CodeJWTVerifyFailed", CodeJWTVerifyFailed},
		{"CodeJWTRefreshFailed", CodeJWTRefreshFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.GreaterOrEqual(t, tt.code, 40000)
			assert.LessOrEqual(t, tt.code, 40099)
		})
	}
}

// TestPredefinedErrors 验证预定义错误构造器的错误码和公开消息。
func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name       string
		builder    goerror.Builder
		wantCode   int
		wantPublic string
	}{
		{"ErrTokenInvalid", ErrTokenInvalid, CodeTokenInvalid, "token_invalid"},
		{"ErrTokenExpired", ErrTokenExpired, CodeTokenExpired, "token_expired"},
		{"ErrTokenRevoked", ErrTokenRevoked, CodeTokenRevoked, "token_revoked"},
		{"ErrDeviceKicked", ErrDeviceKicked, CodeDeviceKicked, "device_kicked"},
		{"ErrSessionInvalid", ErrSessionInvalid, CodeSessionInvalid, "session_invalid"},
		{"ErrSessionExpired", ErrSessionExpired, CodeSessionExpired, "session_expired"},
		{"ErrJWTSignFailed", ErrJWTSignFailed, CodeJWTSignFailed, "jwt_sign_failed"},
		{"ErrJWTVerifyFailed", ErrJWTVerifyFailed, CodeJWTVerifyFailed, "jwt_verify_failed"},
		{"ErrJWTRefreshFailed", ErrJWTRefreshFailed, CodeJWTRefreshFailed, "jwt_refresh_failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.builder.Wrap(errors.New("inner"))
			code, public := goerror.Extract(err)
			assert.Equal(t, tt.wantCode, code)
			assert.Equal(t, tt.wantPublic, public)
		})
	}
}

// TestPredefinedErrors_NonAuthError 验证非认证错误不匹配认证错误码。
func TestPredefinedErrors_NonAuthError(t *testing.T) {
	nonAuthErr := errors.New("some other error")
	code, _ := goerror.Extract(nonAuthErr)
	assert.Equal(t, 0, code)
}
