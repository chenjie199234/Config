package ecode

import (
	cerror "github.com/chenjie199234/Corelib/util/error"
)

var (
	ErrUnknown   = cerror.ErrUnknown //10000
	ErrReq       = cerror.ErrReq     //10001
	ErrResp      = cerror.ErrResp    //10002
	ErrSystem    = cerror.ErrSystem  //10003
	ErrBusiness1 = cerror.MakeError(10004, "business error 1")
)
