package hertz

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
)

func TestRPCErrorRouter_ParamInvalid(t *testing.T) {
	router := &RPCErrorRouter{}
	err := frameworkerror.ErrParamInvalid.Wrap(errors.New("field 'email' is empty"))

	route, ok := router.Route(context.Background(), err)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, route.HTTPCode)
	assert.Equal(t, frameworkerror.CodeParamInvalid, route.BizCode)
	assert.Equal(t, "param_invalid", route.Override)
}

func TestRPCErrorRouter_AuthFailed(t *testing.T) {
	router := &RPCErrorRouter{}
	err := frameworkerror.ErrAuthFailed.Wrap(errors.New("token expired"))

	route, ok := router.Route(context.Background(), err)
	assert.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, route.HTTPCode)
	assert.Equal(t, frameworkerror.CodeAuthFailed, route.BizCode)
}

func TestRPCErrorRouter_Timeout(t *testing.T) {
	router := &RPCErrorRouter{}
	err := frameworkerror.ErrRPCTimeout.Wrap(errors.New("deadline exceeded"))

	route, ok := router.Route(context.Background(), err)
	assert.True(t, ok)
	assert.Equal(t, http.StatusGatewayTimeout, route.HTTPCode)
	assert.Equal(t, frameworkerror.CodeRPCTimeout, route.BizCode)
}

func TestRPCErrorRouter_NonOopsError(t *testing.T) {
	router := &RPCErrorRouter{}
	err := errors.New("plain error")

	route, ok := router.Route(context.Background(), err)
	assert.False(t, ok)
	assert.Equal(t, ErrorRoute{}, route)
}

func TestRPCErrorRouter_NilError(t *testing.T) {
	router := &RPCErrorRouter{}

	route, ok := router.Route(context.Background(), nil)
	assert.False(t, ok)
	assert.Equal(t, ErrorRoute{}, route)
}

func TestNewResponder_Defaults(t *testing.T) {
	r := NewResponder()
	assert.NotNil(t, r)
	assert.False(t, r.debug)
	assert.Equal(t, "X-Request-ID", r.reqIDHeader)
	assert.Equal(t, "Accept-Language", r.langHeader)
	assert.Nil(t, r.translator)
	assert.Nil(t, r.errorRouter)
	assert.Equal(t, http.StatusOK, r.successCode)
	assert.Equal(t, http.StatusInternalServerError, r.failCode)
	assert.NotNil(t, r.reqIDGen)
}

func TestNewResponder_WithDebug(t *testing.T) {
	r := NewResponder(WithDebug(true))
	assert.True(t, r.debug)
}

func TestNewResponder_WithRequestIDHeader(t *testing.T) {
	r := NewResponder(WithRequestIDHeader("X-Custom-ID"))
	assert.Equal(t, "X-Custom-ID", r.reqIDHeader)
}

func TestNewResponder_WithLangHeader(t *testing.T) {
	r := NewResponder(WithLangHeader("X-Lang"))
	assert.Equal(t, "X-Lang", r.langHeader)
}

func TestNewResponder_WithErrorRouter(t *testing.T) {
	router := &RPCErrorRouter{}
	r := NewResponder(WithErrorRouter(router))
	assert.Equal(t, router, r.errorRouter)
}

type mockTranslator struct {
	translate func(ctx context.Context, lang, key string) string
}

func (m *mockTranslator) Translate(ctx context.Context, lang, key string) string {
	return m.translate(ctx, lang, key)
}

func TestNewResponder_WithTranslator(t *testing.T) {
	tr := &mockTranslator{
		translate: func(ctx context.Context, lang, key string) string { return "已翻译" },
	}
	r := NewResponder(WithTranslator(tr))
	assert.NotNil(t, r.translator)
}

func TestNewResponder_WithDefaultBizCode(t *testing.T) {
	r := NewResponder(WithDefaultBizCode(intPtr(200), intPtr(-1)))
	assert.Equal(t, 200, r.successCode)
	assert.Equal(t, -1, r.failCode)
}

func TestNewResponder_WithDefaultBizCode_NilKeepsDefault(t *testing.T) {
	r := NewResponder(WithDefaultBizCode(nil, nil))
	assert.Equal(t, http.StatusOK, r.successCode)
	assert.Equal(t, http.StatusInternalServerError, r.failCode)
}

func TestNewResponder_WithDefaultBizCode_ZeroAllowed(t *testing.T) {
	r := NewResponder(WithDefaultBizCode(intPtr(0), intPtr(0)))
	assert.Equal(t, 0, r.successCode)
	assert.Equal(t, 0, r.failCode)
}

func TestNewResponder_WithDefaultLang(t *testing.T) {
	r := NewResponder(WithDefaultLang("en"))
	assert.Equal(t, "en", r.defaultLang)
}

func TestNewResponder_WithRequestIDGenerator(t *testing.T) {
	r := NewResponder(WithRequestIDGenerator(func() string { return "fixed-id" }))
	assert.Equal(t, "fixed-id", r.reqIDGen())
}

func TestNewResponder_EmptyRequestIDHeader_Disables(t *testing.T) {
	r := NewResponder(WithRequestIDHeader(""))
	assert.Equal(t, "", r.reqIDHeader)
}

func TestResponder_Middleware_Exists(t *testing.T) {
	r := NewResponder()
	handler := r.Middleware()
	assert.NotNil(t, handler)
}
