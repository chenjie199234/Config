package sdk

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/chenjie199234/Config/api"
	"github.com/chenjie199234/Corelib/log"
	"github.com/chenjie199234/Corelib/rpc"
	"github.com/chenjie199234/Corelib/web"
)

type sdk struct {
	path  string
	opnum int64
}

var instance *sdk

//this will create 2 file AppConfig.json and SourceConfig.json in the path and hot update it
//timeout : each call's timeout
//interval: frequence to check update from the config server,this is only useful when watch is disable
func NewRpcSdk(timeout, interval time.Duration, path string, selfgroup, selfname string) error {
	if !atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&instance)), nil, unsafe.Pointer(&sdk{
		path:  path,
		opnum: 0,
	})) {
		return nil
	}
	c, e := api.NewSconfigRpcClient(&rpc.ClientConfig{ConnTimeout: timeout, GlobalTimeout: timeout}, selfgroup, selfname)
	if e != nil {
		return e
	}
	count := 0
	for {
		sleeptime := 50 + 20*count
		if sleeptime > 100 {
			sleeptime = 100
		}
		time.Sleep(time.Millisecond * time.Duration(sleeptime))
		//background context will use the client's timeout
		resp, e := c.Sinfo(context.Background(), &api.Sinforeq{Groupname: selfgroup, Appname: selfname, OpNum: instance.opnum})
		if e != nil {
			log.Error("[config.sdk.rpc] get config info error:", e)
			count++
			if count > 10 {
				return errors.New("[config.sdk.rpc] init config sdk error:" + e.Error())
			}
			continue
		}
		if e := instance.updateAppConfig(resp.CurAppConfig); e != nil {
			return errors.New("[config.sdk.rpc] write appconfig file error:" + e.Error())
		}
		if e := instance.updateSourceConfig(resp.CurSourceConfig); e != nil {
			return errors.New("[config.sdk.rpc] write sourceconfig file error:" + e.Error())
		}
		instance.opnum = resp.OpNum
		break
	}
	//init success
	//start hot update
	go func() {
		tker := time.NewTicker(interval)
		for {
			<-tker.C
			//background context will use the client's timeout
			resp, e := c.Sinfo(context.Background(), &api.Sinforeq{Groupname: selfgroup, Appname: selfname, OpNum: instance.opnum})
			if e != nil {
				log.Error("[config.sdk.rpc] get config info error:", e)
				continue
			}
			if resp.OpNum != instance.opnum {
				if e := instance.updateAppConfig(resp.CurAppConfig); e == nil {
					instance.opnum = resp.OpNum
				}
			}
		}
	}()
	return nil
}

//this will create 2 file AppConfig.json and SourceConfig.json in the path and hot update it
//timeout : each call's timeout
//interval: frequence to check update from the config server,this is only useful when watch is disable
func NewWebSdk(timeout, interval time.Duration, path string, selfgroup, selfname string) error {
	if !atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&instance)), nil, unsafe.Pointer(&sdk{
		path:  path,
		opnum: 0,
	})) {
		return nil
	}
	c, e := api.NewSconfigWebClient(&web.ClientConfig{GlobalTimeout: timeout, IdleTimeout: time.Second * 5, HeartProbe: time.Millisecond * 1500}, selfgroup, selfname)
	if e != nil {
		return nil
	}
	count := 0
	for {
		sleeptime := 50 + 20*count
		if sleeptime > 100 {
			sleeptime = 100
		}
		time.Sleep(time.Millisecond * time.Duration(sleeptime))
		//background context will use the client's timeout
		resp, e := c.Sinfo(context.Background(), &api.Sinforeq{Groupname: selfgroup, Appname: selfname, OpNum: instance.opnum})
		if e != nil {
			log.Error("[config.sdk.web] get config info error:", e)
			count++
			if count > 10 {
				return errors.New("[config.sdk.web] init config sdk error:" + e.Error())
			}
			continue
		}
		if e := instance.updateAppConfig(resp.CurAppConfig); e != nil {
			return errors.New("[config.sdk.web] write appconfig file error:" + e.Error())
		}
		if e := instance.updateSourceConfig(resp.CurSourceConfig); e != nil {
			return errors.New("[config.sdk.web] write sourceconfig file error:" + e.Error())
		}
		instance.opnum = resp.OpNum
		break
	}
	//init success
	//start hot update
	go func() {
		tker := time.NewTicker(interval)
		for {
			<-tker.C
			//background context will use the client's timeout
			resp, e := c.Sinfo(context.Background(), &api.Sinforeq{Groupname: selfgroup, Appname: selfname, OpNum: instance.opnum})
			if e != nil {
				log.Error("[config.sdk.web] get config info error:", e)
				continue
			}
			if resp.OpNum != instance.opnum {
				if e := instance.updateAppConfig(resp.CurAppConfig); e == nil {
					instance.opnum = resp.OpNum
				}
			}
		}
	}()
	return nil
}
func (s *sdk) updateAppConfig(appconfig string) error {
	appfile, e := os.OpenFile(s.path+"/AppConfig_tmp.json", os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if e != nil {
		log.Error("[config.sdk.updatefile] open appconfig file error:", e)
		return e
	}

	n, e := appfile.WriteString(appconfig)
	if e != nil {
		log.Error("[config.sdk.updatefile] write appconfig file error:", e)
		return e
	}
	if n != len(appconfig) {
		log.Error("[config.sdk.updatefile] write appconfig file error: short write")
		return e
	}
	if e = appfile.Sync(); e != nil {
		log.Error("[config.sdk.updatefile] sync appconfig data to disk error:", e)
		return e
	}
	if e = appfile.Close(); e != nil {
		log.Error("[config.sdk.updatefile] close appconfig file error:", e)
		return e
	}
	if e = os.Rename(s.path+"/AppConfig_tmp.json", s.path+"/AppConfig.json"); e != nil {
		log.Error("[config.sdk.updatefile] rename AppConfig_tmp.json to AppConfig.json error:", e)
		return e
	}
	return nil
}
func (s *sdk) updateSourceConfig(sourceconfig string) error {
	sourcefile, e := os.OpenFile(s.path+"/SourceConfig_tmp.json", os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if e != nil {
		log.Error("[config.sdk.updatefile] open sourceconfig file error:", e)
		return e
	}
	n, e := sourcefile.WriteString(sourceconfig)
	if e != nil {
		log.Error("[config.sdk.updatefile] write sourceconfig file error:", e)
		return e
	}
	if n != len(sourceconfig) {
		log.Error("[config.sdk.updatefile] write sourceconfig file error: short write")
		return e
	}
	if e = sourcefile.Sync(); e != nil {
		log.Error("[config.sdk.updatefile] sync sourceconfig data to disk error:", e)
		return e
	}
	if e = sourcefile.Close(); e != nil {
		log.Error("[config.sdk.updatefile] close sourceconfig file error:", e)
		return e
	}
	if e = os.Rename(s.path+"/SourceConfig_tmp.json", s.path+"/SourceConfig.json"); e != nil {
		log.Error("[config.sdk.updatefile] rename SourceConfig_tmp.json to SourceConfig.json error:", e)
		return e
	}
	return nil
}
