package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccessLog_ReturnsHandlerFunc(t *testing.T) {
	handler := AccessLog()
	assert.NotNil(t, handler, "AccessLog should return a HandlerFunc")
}
