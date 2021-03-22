package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"config/config"
	"config/server/xrpc"
	"config/server/xweb"
	"config/service"

	"github.com/chenjie199234/Corelib/discovery"
	"github.com/chenjie199234/Corelib/log"
)

func main() {
	defer config.Close()
	//start the whole business service
	if e := service.StartService(); e != nil {
		log.Error(e)
		return
	}
	//start low level net service
	ch := make(chan os.Signal, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		xrpc.StartRpcServer()
		select {
		case ch <- syscall.SIGTERM:
		default:
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		xweb.StartWebServer()
		select {
		case ch <- syscall.SIGTERM:
		default:
		}
		wg.Done()
	}()
	//try to register self to the discovery server
	stop := make(chan struct{})
	go func(){
		//delay 200ms to register self,if error happened in this 200ms,this server will not be registered
		tmer := time.NewTimer(time.Millisecond * 200)
		select {
		case <-tmer.C:
			rpcc := config.GetRpcConfig()
			webc := config.GetWebConfig()
			regmsg := &discovery.RegMsg{}
			if webc != nil {
				if webc.WebKeyFile != "" && webc.WebCertFile != "" {
					regmsg.WebScheme = "https"
				} else {
					regmsg.WebScheme = "http"
				}
				regmsg.WebPort = 8000
			}
			if rpcc != nil {
				regmsg.RpcPort = 9000
			}
			discovery.RegisterSelf(regmsg)
		case <-stop:
		}
	}()
	signal.Notify(ch, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-ch
	close(stop)
	//stop the whole business service
	service.StopService()
	//stop low level net service
	//grpc server,if don't need,please comment this
	wg.Add(1)
	go func() {
		xrpc.StopRpcServer()
		wg.Done()
	}()
	//http server,if don't need,please comment this
	wg.Add(1)
	go func() {
		xweb.StopWebServer()
		wg.Done()
	}()
	wg.Wait()
}