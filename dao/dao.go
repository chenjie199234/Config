package dao

import (
	"time"

	//"github.com/chenjie199234/Config/api"
	//example "github.chenjie199234/Config/api/deps/example"
	"github.com/chenjie199234/Config/config"
	"github.com/chenjie199234/Corelib/rpc"
	"github.com/chenjie199234/Corelib/web"
)

//var ExampleRpcApi example.ExampleRpcClient
//var ExampleWebApi example.ExampleWebClient

//NewApi create all dependent service's api we need in this program
//example grpc client,http client
func NewApi() error {
	var e error
	_ = e //avoid unuse

	rpcc := getRpcClientConfig()
	_ = rpcc //avoid unuse

	//init rpc client below
	//if ExampleRpcApi, e = example.NewExampleRpcClient(rpcc, api.Group, api.Name); e != nil {
	//        return e
	//}

	webc := getWebClientConfig()
	_ = webc //avoid unuse

	//init web client below
	//if ExampleWebApi, e = example.NewExampleWebClient(webc, api.Group, api.Name); e != nil {
	//        return e
	//}
	return nil
}
func getRpcClientConfig() *rpc.ClientConfig {
	rc := config.GetRpcClientConfig()
	rpcverifydata := ""
	if len(config.EC.ServerVerifyDatas) != 0 {
		rpcverifydata = config.EC.ServerVerifyDatas[0]
	}
	return &rpc.ClientConfig{
		ConnTimeout:            time.Duration(rc.ConnTimeout),
		GlobalTimeout:          time.Duration(rc.GlobalTimeout),
		HeartTimeout:           time.Duration(rc.HeartTimeout),
		HeartPorbe:             time.Duration(rc.HeartProbe),
		GroupNum:               1,
		SocketRBuf:             1024,
		SocketWBuf:             1024,
		MaxMsgLen:              65535,
		MaxBufferedWriteMsgNum: 256,
		VerifyData:             rpcverifydata,
	}
}
func getWebClientConfig() *web.ClientConfig {
	wc := config.GetWebClientConfig()
	return &web.ClientConfig{
		GlobalTimeout: time.Duration(wc.GlobalTimeout),
		IdleTimeout:   time.Duration(wc.IdleTimeout),
		HeartProbe:    time.Duration(wc.HeartProbe),
		MaxHeader:     1024,
		SocketRBuf:    1024,
		SocketWBuf:    1024,
		SkipVerifyTLS: wc.SkipVerifyTls,
		CAs:           wc.Cas,
	}
}
