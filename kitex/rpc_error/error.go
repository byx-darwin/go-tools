package rpc_error

import "github.com/cloudwego/kitex/pkg/kerrors"

type ErrorType int

const (
	ErrorTypeInvalid ErrorType = iota
	// ErrorTypeDBDataRepeat 数据重复错误
	ErrorTypeDBDataRepeat
	// ErrorTypeDataInvalid 数据无效
	ErrorTypeDataInvalid
	// ErrorTypeSqlInsertFailure sql 插入失败
	ErrorTypeSqlInsertFailure
	// ErrorTypeSqlUpdateFailure sql 更新失败
	ErrorTypeSqlUpdateFailure
	// ErrorTypeSqlDeleteFailure sql 删除失败
	ErrorTypeSqlDeleteFailure
	// ErrorTypeSqlQueryFailure sql 查询失败
	ErrorTypeSqlQueryFailure
	// ErrorTypeSqlExecFailure sql 执行失败
	ErrorTypeSqlExecFailure
	// ErrorTypeSqlTxFailure sql 事务失败
	ErrorTypeSqlTxFailure
	// ErrorTypeSqlTxCommitFailure sql 事务提交失败
	ErrorTypeSqlTxCommitFailure
	// ErrorTypeSqlTxRollbackFailure sql 事务回滚失败
	ErrorTypeSqlTxRollbackFailure
	// ErrorTypeSqlTxBeginFailure sql 事务开始失败
	ErrorTypeSqlTxBeginFailure
	// ErrorTypeNetwork network 网络错误
	ErrorTypeNetwork
	// ErrorTypeJsonMarshal JsonMarshal json 转换错误
	ErrorTypeJsonMarshal
	// ErrorTypeJsonUnMarshal UnMarshal json 解析错误
	ErrorTypeJsonUnMarshal
	// ErrorTypeDataEmpty 数据为空
	ErrorTypeDataEmpty
	// ErrTypeRpcError RPC 网络层错误
	ErrTypeRpcError
)

func NewBizStatusError(code ErrorType, err error) kerrors.BizStatusErrorIface {
	return kerrors.NewBizStatusError(int32(code), err.Error())
}

func ParseBizStatusError(err error) (errType ErrorType, privateErrMsg string) {
	if bizErr, ok := kerrors.FromBizStatusError(err); ok {
		code := ErrorType(bizErr.BizStatusCode())
		if code > ErrorTypeDataInvalid {
			switch code {
			case ErrorTypeSqlInsertFailure:
				privateErrMsg += "sql insert,reason:"
			case ErrorTypeSqlUpdateFailure:
				privateErrMsg += "sql update,reason:"
			case ErrorTypeSqlDeleteFailure:
				privateErrMsg += "sql delete,reason:"
			case ErrorTypeSqlQueryFailure:
				privateErrMsg += "sql query,reason:"
			case ErrorTypeSqlExecFailure:
				privateErrMsg += "sql exec,reason:"
			case ErrorTypeSqlTxFailure:
				privateErrMsg += "sql tx,reason:"
			case ErrorTypeSqlTxCommitFailure:
				privateErrMsg += "sql tx commit,reason:"
			case ErrorTypeSqlTxRollbackFailure:
				privateErrMsg += "sql tx rollback,reason:"
			case ErrorTypeSqlTxBeginFailure:
				privateErrMsg += "sql tx begin,reason:"
			case ErrorTypeNetwork:
				privateErrMsg += "network Error,reason:"
			case ErrorTypeJsonMarshal:
				privateErrMsg += "JsonMarshal Error,reason:"
			case ErrorTypeJsonUnMarshal:
				privateErrMsg += "JsonUnMarshal Error,reason:"
			case ErrorTypeDataEmpty:
				privateErrMsg += "date empty,reason:"
			default:
				privateErrMsg += "unknown error,reason:"
			}
		}
		errType = code
		privateErrMsg += bizErr.BizMessage()
	} else {
		errType = ErrTypeRpcError
		privateErrMsg += err.Error()
	}
	return
}
