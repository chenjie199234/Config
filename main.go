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
	registerwg := &sync.WaitGroup{}
	registerwg.Add(1)
	wg.Add(1)
	go func() {
		xrpc.StartRpcServer(registerwg)
		select {
		case ch <- syscall.SIGTERM:
		default:
		}
		wg.Done()
	}()
	registerwg.Add(1)
	wg.Add(1)
	go func() {
		xweb.StartWebServer(registerwg)
		select {
		case ch <- syscall.SIGTERM:
		default:
		}
		wg.Done()
	}()
	//try to register self to the discovery server
	stopreg := make(chan struct{})
	if config.EC.ServerVerifyDatas != nil {
		go func() {
			registerwg.Wait()
			//delay 200ms,wait rpc and web server to start their tcp listener
			//if error happened in this 200ms,server will not be registered
			tmer := time.NewTimer(time.Millisecond * 200)
			select {
			case <-tmer.C:
				if e := discoverysdk.RegisterSelf(nil); e != nil {
					log.Error("[main] register self to discovery server error:", e)
					select {
					case ch <- syscall.SIGTERM:
					default:
					}
				}
			case <-stopreg:
			}
		}()
	}
	signal.Notify(ch, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-ch
	close(stopreg)
	//unregisterself
	discoverysdk.UnRegisterSelf()
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
