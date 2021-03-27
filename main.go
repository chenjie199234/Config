package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/chenjie199234/Config/config"
	"github.com/chenjie199234/Config/server/xrpc"
	"github.com/chenjie199234/Config/server/xweb"
	"github.com/chenjie199234/Config/service"
	"github.com/chenjie199234/Corelib/log"
	discoverysdk "github.com/chenjie199234/Discovery/sdk"
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
	go func() {
		//delay 200ms to register self,if error happened in this 200ms,this server will not be registered
		tmer := time.NewTimer(time.Millisecond * 200)
		select {
		case <-tmer.C:
			webc := config.GetWebServerConfig()
			if webc != nil && len(webc.CertKey) > 0 {
				discoverysdk.RegisterSelf(nil)
			} else {
				discoverysdk.RegisterSelf(nil)
			}
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
