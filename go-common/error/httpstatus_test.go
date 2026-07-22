package error

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// 测试专用码使用 39900-39989 未分配段，避免与各模块 init() 注册的真实码冲突。

func TestRegisterHTTPStatuses_Lookup(t *testing.T) {
	RegisterHTTPStatuses(map[int]int{39901: 418})
	err := Code(39901).Public("teapot").Wrap(errors.New("x"))
	assert.Equal(t, 418, HTTPStatus(err))
}

func TestRegisterHTTPStatuses_RegistryPrecedence(t *testing.T) {
	// 未注册时 39903 走兜底（迁移期内置 switch default → 200）；注册后应返回注册值。
	RegisterHTTPStatuses(map[int]int{39903: 503})
	err := Code(39903).Public("svc_down").Wrap(errors.New("x"))
	assert.Equal(t, 503, HTTPStatus(err))
}

func TestRegisterHTTPStatuses_DuplicatePanics(t *testing.T) {
	RegisterHTTPStatuses(map[int]int{39902: 500})
	assert.Panics(t, func() {
		RegisterHTTPStatuses(map[int]int{39902: 503})
	})
}
