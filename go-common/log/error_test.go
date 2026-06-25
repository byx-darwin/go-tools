package log_test

import (
	"errors"
	"testing"

	"github.com/byx-darwin/go-tools/go-common/log"
	"github.com/samber/oops"
	"github.com/stretchr/testify/require"
)

func TestErrorAttrs_OopsError(t *testing.T) {
	err := oops.Code("DB_ERROR").Wrap(errors.New("connection refused"))
	attrs := log.ErrorAttrs(err)
	require.NotEmpty(t, attrs)
}

func TestErrorAttrs_RegularError(t *testing.T) {
	err := errors.New("regular error")
	attrs := log.ErrorAttrs(err)
	require.Empty(t, attrs)
}

func TestErrorAttrs_NilError(t *testing.T) {
	attrs := log.ErrorAttrs(nil)
	require.Empty(t, attrs)
}

func TestErrorAttrs_OopsError_CodeOnly(t *testing.T) {
	err := oops.Code("TIMEOUT").Wrap(errors.New("timeout"))
	attrs := log.ErrorAttrs(err)
	require.NotEmpty(t, attrs)
}
