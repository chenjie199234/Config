package xrpc

import (
	"sync"
	"time"

	"github.com/chenjie199234/Config/api"
	"github.com/chenjie199234/Config/config"
	"github.com/chenjie199234/Config/service"
	"github.com/chenjie199234/Corelib/log"
	"github.com/chenjie199234/Corelib/rpc"
	"github.com/chenjie199234/Corelib/rpc/mids"
	discoverysdk "github.com/chenjie199234/Discovery/sdk"
)

var s *rpc.RpcServer

//StartRpcServer -
func StartRpcServer(wg *sync.WaitGroup) {
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
	//example
	//if e = api.RegisterExampleRpcServer(s, service.SvcExample,mids.AllMids()); e != nil {
	//log.Error("[xrpc] register handlers error:", e)
	//return
	//}

	if config.EC.ServerVerifyDatas != nil {
		if e = discoverysdk.RegRpc(9000); e != nil {
			log.Error("[xrpc] register rpc to discovery server error:", e)
			return
		}
	}
	wg.Done()
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
