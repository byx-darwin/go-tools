package middleware

import (
	"testing"

	"github.com/byx-darwin/go-tools/go-common/log"
	"github.com/stretchr/testify/assert"
)

func TestAccessLog_ReturnsHandlerFunc(t *testing.T) {
	l := log.NewFromConfig(log.Config{})
	defer l.Close()
	handler := AccessLog(l)
	assert.NotNil(t, handler, "AccessLog should return a HandlerFunc")
}
