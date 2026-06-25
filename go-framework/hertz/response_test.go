package hertz

import (
	"context"
	"errors"
	"net/http"
	"testing"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/stretchr/testify/assert"
)

func TestRPCErrorRouter_ParamInvalid(t *testing.T) {
	router := &RPCErrorRouter{}
	err := goerror.ErrParamInvalid.Wrap(errors.New("field 'email' is empty"))

	route, ok := router.Route(context.Background(), err)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, route.HTTPCode)
	assert.Equal(t, goerror.CodeParamInvalid, route.BizCode)
	assert.Equal(t, "param_invalid", route.Override)
}

func TestRPCErrorRouter_AuthFailed(t *testing.T) {
	router := &RPCErrorRouter{}
	err := goerror.ErrAuthFailed.Wrap(errors.New("token expired"))

	route, ok := router.Route(context.Background(), err)
	assert.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, route.HTTPCode)
	assert.Equal(t, goerror.CodeAuthFailed, route.BizCode)
}

func TestRPCErrorRouter_Timeout(t *testing.T) {
	router := &RPCErrorRouter{}
	err := goerror.ErrRPCTimeout.Wrap(errors.New("deadline exceeded"))

	route, ok := router.Route(context.Background(), err)
	assert.True(t, ok)
	assert.Equal(t, http.StatusGatewayTimeout, route.HTTPCode)
	assert.Equal(t, goerror.CodeRPCTimeout, route.BizCode)
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
