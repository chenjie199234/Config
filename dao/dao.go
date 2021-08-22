package dao

import (
	"net"
	"time"

	//"githhub.com/chenjie199234/Config/api"
	//example "github.com/chenjie199234/Config/api/deps/example"
	"github.com/chenjie199234/Config/config"

	"github.com/chenjie199234/Corelib/log"
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
		DiscoverFunction:       rpcDNS,
		DiscoverInterval:       time.Duration(rc.DiscoverInterval),
	}
}
func getWebClientConfig() *web.ClientConfig {
	wc := config.GetWebClientConfig()
	return &web.ClientConfig{
		GlobalTimeout:    time.Duration(wc.GlobalTimeout),
		IdleTimeout:      time.Duration(wc.IdleTimeout),
		HeartProbe:       time.Duration(wc.HeartProbe),
		MaxHeader:        1024,
		SocketRBuf:       1024,
		SocketWBuf:       1024,
		SkipVerifyTLS:    wc.SkipVerifyTls,
		DiscoverFunction: webDNS,
		DiscoverInterval: time.Duration(wc.DiscoverInterval),
	}
}

func webDNS(group, name string) (map[string]*web.RegisterData, error) {
	result := make(map[string]*web.RegisterData)
	addrs, e := net.LookupHost(name + "-service-headless" + "." + group)
	if e != nil {
		log.Error("[web.dns] get:", name+"-service-headless", "addrs error:", e)
		return nil, e
	}
	for i := range addrs {
		addrs[i] = "http://" + addrs[i] + ":8000"
	}
	dserver := make(map[string]struct{})
	dserver["dns"] = struct{}{}
	for _, addr := range addrs {
		result[addr] = &web.RegisterData{DServers: dserver}
	}
	return result, nil
}

func rpcDNS(group, name string) (map[string]*rpc.RegisterData, error) {
	result := make(map[string]*rpc.RegisterData)
	addrs, e := net.LookupHost(name + "-service-headless" + "." + group)
	if e != nil {
		log.Error("[rpc.dns] get:", name+"-service-headless", "addrs error:", e)
		return nil, e
	}
	for i := range addrs {
		addrs[i] = addrs[i] + ":9000"
	}
	dserver := make(map[string]struct{})
	dserver["dns"] = struct{}{}
	for _, addr := range addrs {
		result[addr] = &rpc.RegisterData{DServers: dserver}
	}
	return result, nil
}
