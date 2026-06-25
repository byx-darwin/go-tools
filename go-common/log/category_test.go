package log_test

import (
	"testing"

	"github.com/byx-darwin/go-tools/go-common/log"
	"github.com/stretchr/testify/require"
)

func TestCategoryConstants(t *testing.T) {
	require.Equal(t, "access", log.CategoryAccess)
	require.Equal(t, "error", log.CategoryError)
	require.Equal(t, "biz", log.CategoryBiz)
	require.Equal(t, "rpc", log.CategoryRPC)
	require.Equal(t, "db", log.CategoryDB)
	require.Equal(t, "panic", log.CategoryPanic)
	require.Equal(t, "audit", log.CategoryAudit)
	require.Equal(t, "security", log.CategorySecurity)
}
