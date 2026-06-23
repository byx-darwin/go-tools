package rpcerror

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorType_Values(t *testing.T) {
	assert.Equal(t, ErrorType(0), ErrorTypeInvalid)
	assert.Equal(t, ErrorType(1), ErrorTypeDataRepeat)
	assert.Equal(t, ErrorType(2), ErrorTypeDataInvalid)
	assert.Equal(t, ErrorType(17), ErrorTypeDataEmpty)
	assert.Equal(t, ErrorType(18), ErrorTypeRPCError)
}

func TestBizStatusError_Error(t *testing.T) {
	e := NewBizStatusError(ErrorTypeSQLExecFailure, "insert user failed", errors.New("connection refused"))
	msg := e.Error()
	assert.Contains(t, msg, "insert user failed")
	assert.Contains(t, msg, "connection refused")
}

func TestBizStatusError_Error_NoWrap(t *testing.T) {
	e := NewBizStatusError(ErrorTypeDataEmpty, "no records found", nil)
	msg := e.Error()
	assert.Contains(t, msg, "no records found")
	assert.NotContains(t, msg, "<nil>")
}

func TestBizStatusError_BizStatusCode(t *testing.T) {
	e := NewBizStatusError(ErrorTypeParamInvalid, "bad param", nil)
	assert.Equal(t, int32(4), e.BizStatusCode())
}

func TestBizStatusError_BizMessage(t *testing.T) {
	e := NewBizStatusError(ErrorTypeNetwork, "timeout", nil)
	assert.Equal(t, "timeout", e.BizMessage())
}

func TestBizStatusError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	e := NewBizStatusError(ErrorTypeJSONMarshal, "marshal failed", inner)
	assert.True(t, errors.Is(e, inner))
}

func TestParseBizStatusError_Nil(t *testing.T) {
	code, msg := ParseBizStatusError(nil)
	assert.Equal(t, ErrorType(0), code)
	assert.Empty(t, msg)
}

func TestParseBizStatusError_BizStatus(t *testing.T) {
	e := NewBizStatusError(ErrorTypeDataRepeat, "duplicate key", nil)
	code, msg := ParseBizStatusError(e)
	assert.Equal(t, ErrorTypeDataRepeat, code)
	assert.Equal(t, "duplicate key", msg)
}

func TestParseBizStatusError_Wrapped(t *testing.T) {
	e := NewBizStatusError(ErrorTypeSQLQueryFailure, "query failed", errors.New("db error"))
	wrapped := errors.New("middleware error: " + e.Error())

	// Parse unwraps through *BizStatusError chain
	code, msg := ParseBizStatusError(wrapped)
	// Since *BizStatusError.Error() string is not unwrapable, it falls through to RPCError
	assert.Equal(t, ErrorTypeRPCError, code)
	assert.NotEmpty(t, msg)
}

func TestParseBizStatusError_StandardError(t *testing.T) {
	code, msg := ParseBizStatusError(errors.New("standard error"))
	assert.Equal(t, ErrorTypeRPCError, code)
	assert.Equal(t, "standard error", msg)
}

func TestErrorType_AllEnums(t *testing.T) {
	names := map[ErrorType]string{
		ErrorTypeInvalid:          "Invalid",
		ErrorTypeDataRepeat:       "DataRepeat",
		ErrorTypeDataInvalid:      "DataInvalid",
		ErrorTypeDataOneself:      "DataOneself",
		ErrorTypeParamInvalid:     "ParamInvalid",
		ErrorTypeSQLInsertFailure: "SQLInsertFailure",
		ErrorTypeSQLUpdateFailure: "SQLUpdateFailure",
		ErrorTypeSQLDeleteFailure: "SQLDeleteFailure",
		ErrorTypeSQLQueryFailure:  "SQLQueryFailure",
		ErrorTypeSQLExecFailure:   "SQLExecFailure",
		ErrorTypeSQLTxFailure:     "SQLTxFailure",
		ErrorTypeSQLTxCommitFailure:   "SQLTxCommitFailure",
		ErrorTypeSQLTxRollbackFailure: "SQLTxRollbackFailure",
		ErrorTypeSQLTxBeginFailure:    "SQLTxBeginFailure",
		ErrorTypeNetwork:              "Network",
		ErrorTypeJSONMarshal:          "JSONMarshal",
		ErrorTypeJSONUnmarshal:        "JSONUnmarshal",
		ErrorTypeDataEmpty:            "DataEmpty",
		ErrorTypeRPCError:             "RPCError",
	}

	// All error types should be unique
	seen := make(map[ErrorType]bool)
	for code, name := range names {
		assert.False(t, seen[code], "duplicate error type: %s (code=%d)", name, code)
		seen[code] = true
	}

	assert.Len(t, names, 19, "should have 19 error types")
}
