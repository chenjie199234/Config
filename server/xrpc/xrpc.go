package xrpc

import (
	"time"

	"config/api"
	"config/config"
	"config/service"

	"github.com/chenjie199234/Corelib/log"
	"github.com/chenjie199234/Corelib/rpc"
	"github.com/chenjie199234/Corelib/rpc/mids"
)

var s *rpc.RpcServer

//StartRpcServer -
func StartRpcServer() {
	c := config.GetRpcServerConfig()
	rpcc := &rpc.ServerConfig{
		GlobalTimeout:          time.Duration(c.GlobalTimeout),
		HeartTimeout:           time.Duration(c.HeartTimeout),
		HeartPorbe:             time.Duration(c.HeartProbe),
		GroupNum:               1,
		SocketRBuf:             1024,
		SocketWBuf:             1024,
		MaxMsgLen:              65535,
		MaxBufferedWriteMsgNum: 1024,
		VerifyDatas:            config.EC.ServerVerifyDatas,
	}
	var e error
	if s, e = rpc.NewRpcServer(rpcc, api.Group, api.Name); e != nil {
		log.Error("[xrpc] new error:", e)
		return
	}

	//this place can register global midwares
	//s.Use(globalmidwares)

	//you just need to register your service here
	if e = api.RegisterStatusRpcServer(s, service.SvcStatus, mids.AllMids()); e != nil {
		log.Error("[xrpc] register handlers error:", e)
		return
	}
	if e = api.RegisterSconfigRpcServer(s, service.SvcSconfig, mids.AllMids()); e != nil {
		log.Error("[xrpc] register handlers error:", e)
		return
	}
	if e = api.RegisterCconfigRpcServer(s, service.SvcCconfig, mids.AllMids()); e != nil {
		log.Error("[xrpc] register handlers error:", e)
		return
	}
	//example
	//if e = api.RegisterExampleRpcServer(s, service.SvcExample,mids.AllMids()); e != nil {
	//log.Error("[xrpc] register handlers error:", e)
	//return
	//}

	if e = s.StartRpcServer(":9000"); e != nil {
		log.Error("[xrpc] start error:", e)
		return
	}
}

//StopRpcServer -
func StopRpcServer() {
	if s != nil {
		s.StopRpcServer()
	}
}

