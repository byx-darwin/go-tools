// Package rpcerror 提供 Kitex RPC 的错误类型定义和 oops 桥接。
package rpcerror

import "fmt"

// ErrorType 业务错误类型枚举
type ErrorType int32

const (
	ErrorTypeInvalid          ErrorType = iota
	ErrorTypeDataRepeat                 // 数据重复
	ErrorTypeDataInvalid                // 数据无效
	ErrorTypeDataOneself                // 数据自身
	ErrorTypeParamInvalid               // 参数无效
	ErrorTypeSQLInsertFailure           // SQL 插入失败
	ErrorTypeSQLUpdateFailure           // SQL 更新失败
	ErrorTypeSQLDeleteFailure           // SQL 删除失败
	ErrorTypeSQLQueryFailure            // SQL 查询失败
	ErrorTypeSQLExecFailure             // SQL 执行失败
	ErrorTypeSQLTxFailure               // SQL 事务失败
	ErrorTypeSQLTxCommitFailure         // SQL 事务提交失败
	ErrorTypeSQLTxRollbackFailure       // SQL 事务回滚失败
	ErrorTypeSQLTxBeginFailure          // SQL 事务开始失败
	ErrorTypeNetwork                    // 网络错误
	ErrorTypeJSONMarshal                // JSON 序列化错误
	ErrorTypeJSONUnmarshal              // JSON 反序列化错误
	ErrorTypeDataEmpty                  // 数据为空
	ErrorTypeRPCError                   // RPC 网络层错误
)

// Statuser 定义 BizStatusError 接口（兼容 kitex kerrors）
type Statuser interface {
	BizStatusCode() int32
	BizMessage() string
}

// BizStatusError 业务状态错误
type BizStatusError struct {
	Code    ErrorType
	Message string
	Err     error
}

func (e *BizStatusError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *BizStatusError) BizStatusCode() int32 { return int32(e.Code) }
func (e *BizStatusError) BizMessage() string   { return e.Message }
func (e *BizStatusError) Unwrap() error        { return e.Err }

// NewBizStatusError 创建业务状态错误
func NewBizStatusError(code ErrorType, msg string, err error) *BizStatusError {
	return &BizStatusError{Code: code, Message: msg, Err: err}
}

// ParseBizStatusError 从 error 中提取 ErrorType 和消息
func ParseBizStatusError(err error) (ErrorType, string) {
	if err == nil {
		return 0, ""
	}

	// Try oops-style unwrapping
	for e := err; e != nil; {
		if biz, ok := e.(*BizStatusError); ok {
			return biz.Code, biz.Message
		}
		e = unwrap(e)
	}

	return ErrorTypeRPCError, err.Error()
}

func unwrap(err error) error {
	type wrapper interface {
		Unwrap() error
	}
	if w, ok := err.(wrapper); ok {
		return w.Unwrap()
	}
	return nil
}
