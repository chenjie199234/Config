package ecode

import (
	cerror "github.com/chenjie199234/Corelib/util/error"
)

var (
	ErrUnknown       = cerror.ErrUnknown //10000
	ErrReq           = cerror.ErrReq     //10001
	ErrResp          = cerror.ErrResp    //10002
	ErrSystem        = cerror.ErrSystem  //10003
	ErrNotExist      = cerror.MakeError(10004, "not exist")
	ErrCoinfigFormat = cerror.MakeError(10005, "config format error: must be json object")
)
