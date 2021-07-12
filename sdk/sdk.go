package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"sort"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/chenjie199234/Config/api"
	"github.com/chenjie199234/Config/dao/sconfig"
	"github.com/chenjie199234/Corelib/log"
	"github.com/chenjie199234/Corelib/rpc"
	"github.com/chenjie199234/Corelib/util/common"
	"github.com/chenjie199234/Corelib/web"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type sdk struct {
	path  string
	opnum int64
}

var instance *sdk

func newWebConfig() *web.ClientConfig {
	webc := &web.ClientConfig{}
	webc.Discover = func(_, _ string, manually <-chan struct{}, client *web.WebClient) {
		host := "config-service-headless.default"
		dserver := make(map[string]struct{}, 2)
		dserver["kubernetesdns"] = struct{}{}
		regdata := &web.RegisterData{
			DServers: dserver,
			Addition: nil,
		}
		current := make([]string, 0)
		finder := func() {
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
			defer cancel()
			addrs, e := net.DefaultResolver.LookupHost(ctx, host)
			if e != nil {
				log.Error("[Config.websdk] dns resolve host:", host, "error:", e)
				return
			}
			if len(addrs) != 0 {
				sort.Strings(addrs)
				for i, addr := range addrs {
					addrs[i] = addr + ":8000"
				}
			} else {
				log.Warning("[Config.websdk] dns resolve host:", host, "return empty result")
			}
			different := false
			if len(addrs) != len(current) {
				different = true
			} else {
				for i, addr := range addrs {
					if addr != current[i] {
						different = true
						break
					}
				}
			}
			if different {
				current = addrs
				log.Info("[Config.websdk] dns resolve host:", host, "result:", current)
				all := make(map[string]*web.RegisterData, len(addrs)+2)
				for _, addr := range addrs {
					all[addr] = regdata
				}
				client.UpdateDiscovery(all)
			}
		}
		finder()
		tker := time.NewTicker(time.Second * 5)
		for {
			select {
			case <-tker.C:
				finder()
			case <-manually:
				finder()
				tker.Reset(time.Second * 5)
				for len(tker.C) > 0 {
					<-tker.C
				}
			}
		}
	}
	return webc
}
func NewWebSdk(path, selfgroup, selfname string, watch bool, loopInterval time.Duration) error {
	if !atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&instance)), nil, unsafe.Pointer(&sdk{
		path:  path,
		opnum: 0,
	})) {
		return nil
	}
	client, e := api.NewSconfigWebClient(newWebConfig(), selfgroup, selfname)
	if e != nil {
		log.Error("[Config.websdk] new config client error:", e)
		return e
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancel()
	if watch {
		var resp *api.Sgetwatchaddrresp
		for {
			//wait for discovery
			time.Sleep(time.Millisecond * 50)
			resp, e = client.Sgetwatchaddr(ctx, &api.Sgetwatchaddrreq{})
			if e != nil {
				log.Error("[Config.websdk] call config server for watch addr error:", e)
				if e == context.DeadlineExceeded || e == context.Canceled {
					return errors.New("[Config.websdk] call config server for watch addr failed")
				}
				continue
			}
			break
		}
		if len(resp.Addrs) == 0 {
			log.Warning("[Config.websdk] init watch error: config server doesn't support watch")
			log.Warning("[Config.websdk] fallback to loop")
		} else if db, e := newmongo(resp.Username, resp.Passwd, resp.ReplicaSetName, resp.Addrs); e != nil {
			log.Warning("[Config.websdk] init watch error:", e)
			log.Warning("[Config.websdk] fallback to loop")
		} else {
			//run watch logic
			dao := sconfig.NewDao(nil, nil, db)
			for {
				sum, conf, e := dao.MongoGetInfo(ctx, selfgroup, selfname, 0)
				if e != nil {
					log.Error("[Config.websdk] get config data from db error:", e)
					if e == context.DeadlineExceeded || e == context.Canceled {
						return errors.New("[Config.websdk] get config data from db failed")
					}
					time.Sleep(time.Millisecond * 50)
					continue
				}
				appc := ""
				sourcec := ""
				if sum == nil || conf == nil {
					instance.opnum = 0
				} else {
					instance.opnum = sum.OpNum
					appc = conf.AppConfig
					sourcec = conf.SourceConfig
				}
				if e := instance.updateAppConfig(appc); e != nil {
					return errors.New("[Config.websdk] write appconfig file error:" + e.Error())
				}
				if e := instance.updateSourceConfig(sourcec); e != nil {
					return errors.New("[Config.websdk] write sourceconfig file error:" + e.Error())
				}
				break
			}
			go func() {
				for {
					if e := dao.MongoWatch(selfgroup, selfname, func(opnum int64, config *sconfig.Config) {
						if opnum != instance.opnum {
							//only appconfig can be hot updated
							var e error
							if config == nil {
								e = instance.updateAppConfig("")
							} else {
								e = instance.updateAppConfig(config.AppConfig)
							}
							if e != nil {
								log.Error("[Config.websdk] hot update write appconfig file error:", e)
							} else {
								instance.opnum = opnum
							}
						}
					}); e != nil {
						log.Error("[Config.websdk] watch mongodb error:", e)
						time.Sleep(time.Second)
					}
				}
			}()
			return nil
		}
	}
	//run loop logic
	for {
		//wait for discovery
		time.Sleep(time.Millisecond * 50)
		resp, e := client.Sinfo(ctx, &api.Sinforeq{Groupname: selfgroup, Appname: selfname, OpNum: instance.opnum})
		if e != nil {
			log.Error("[Config.websdk] call config server for config data error:", e)
			if e == context.DeadlineExceeded || e == context.Canceled {
				return errors.New("[Config.websdk] call config server for config data failed")
			}
			continue
		}
		if e := instance.updateAppConfig(resp.CurAppConfig); e != nil {
			return errors.New("[Config.websdk] write appconfig file error:" + e.Error())
		}
		if e := instance.updateSourceConfig(resp.CurSourceConfig); e != nil {
			return errors.New("[Config.websdk] write sourceconfig file error:" + e.Error())
		}
		instance.opnum = resp.OpNum
		break
	}
	go func() {
		if loopInterval < time.Second {
			//too fast
			//if need fast use watch mode
			loopInterval = time.Second
		}
		tker := time.NewTicker(loopInterval)
		for {
			<-tker.C
			ctx, cancel := context.WithTimeout(context.Background(), loopInterval)
			resp, e := client.Sinfo(ctx, &api.Sinforeq{Groupname: selfgroup, Appname: selfname, OpNum: instance.opnum})
			if e != nil {
				log.Error("[Config.websdk] hot update call config server for config data error:", e)
			} else if resp.OpNum != instance.opnum {
				//only appconfig can be hot updated
				if e := instance.updateAppConfig(resp.CurAppConfig); e != nil {
					log.Error("[Config.websdk] hot update write appconfig file error:", e)
				} else {
					instance.opnum = resp.OpNum
				}
			}
			cancel()
			for len(tker.C) > 0 {
				<-tker.C
			}
		}
	}()
	return nil
}
func newRpcConfig() *rpc.ClientConfig {
	rpcc := &rpc.ClientConfig{}
	rpcc.Discover = func(group, name string, manually <-chan struct{}, client *rpc.RpcClient) {
		host := name + "-service-headless." + group
		dserver := make(map[string]struct{}, 2)
		dserver["kubernetesdns"] = struct{}{}
		regdata := &rpc.RegisterData{
			DServers: dserver,
			Addition: nil,
		}
		current := make([]string, 0)
		finder := func() {
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
			defer cancel()
			addrs, e := net.DefaultResolver.LookupHost(ctx, host)
			if e != nil {
				log.Error("[Config.rpcsdk] dns resolve host:", host, "error:", e)
				return
			}
			if len(addrs) != 0 {
				sort.Strings(addrs)
				for i, addr := range addrs {
					addrs[i] = addr + ":9000"
				}
			} else {
				log.Warning("[Config.rpcsdk] dns resolve host:", host, "return empty result")
			}
			different := false
			if len(addrs) != len(current) {
				different = true
			} else {
				for i, addr := range addrs {
					if addr != current[i] {
						different = true
						break
					}
				}
			}
			if different {
				current = addrs
				log.Info("[Config.rpcsdk] dns resolve host:", host, "result:", current)
				all := make(map[string]*rpc.RegisterData, len(addrs)+2)
				for _, addr := range addrs {
					all[addr] = regdata
				}
				client.UpdateDiscovery(all)
			}
		}
		finder()
		tker := time.NewTicker(time.Second * 5)
		for {
			select {
			case <-tker.C:
				finder()
			case <-manually:
				finder()
				tker.Reset(time.Second * 5)
				for len(tker.C) > 0 {
					<-tker.C
				}
			}
		}
	}
	return rpcc
}
func NewRpcSdk(path, selfgroup, selfname string, watch bool, loopInterval time.Duration) error {
	if !atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&instance)), nil, unsafe.Pointer(&sdk{
		path:  path,
		opnum: 0,
	})) {
		return nil
	}
	client, e := api.NewSconfigRpcClient(newRpcConfig(), selfgroup, selfname)
	if e != nil {
		log.Error("[Config.rpcsdk] new config client error:", e)
		return e
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancel()
	if watch {
		var resp *api.Sgetwatchaddrresp
		for {
			//wait for discovery
			time.Sleep(time.Millisecond * 50)
			resp, e = client.Sgetwatchaddr(ctx, &api.Sgetwatchaddrreq{})
			if e != nil {
				log.Error("[Config.rpcsdk] call config server for watch addr error:", e)
				if e == context.DeadlineExceeded || e == context.Canceled {
					return errors.New("[Config.rpcsdk] call config server for watch addr failed")
				}
				continue
			}
			break
		}
		if len(resp.Addrs) == 0 {
			log.Warning("[Config.rpcsdk] init watch error: config server doesn't support watch")
			log.Warning("[Config.rpcsdk] fallback to loop")
		} else if db, e := newmongo(resp.Username, resp.Passwd, resp.ReplicaSetName, resp.Addrs); e != nil {
			log.Warning("[Config.rpcsdk] init watch error:", e)
			log.Warning("[Config.rpcsdk] fallback to loop")
		} else {
			//run watch logic
			dao := sconfig.NewDao(nil, nil, db)
			for {
				sum, conf, e := dao.MongoGetInfo(ctx, selfgroup, selfname, 0)
				if e != nil {
					log.Error("[Config.rpcsdk] get config data from db error:", e)
					if e == context.DeadlineExceeded || e == context.Canceled {
						return errors.New("[Config.rpcsdk] get config data from db failed")
					}
					time.Sleep(time.Millisecond * 50)
					continue
				}
				appc := ""
				sourcec := ""
				if sum == nil || conf == nil {
					instance.opnum = 0
				} else {
					instance.opnum = sum.OpNum
					appc = conf.AppConfig
					sourcec = conf.SourceConfig
				}
				if e := instance.updateAppConfig(appc); e != nil {
					return errors.New("[Config.rpcsdk] write appconfig file error:" + e.Error())
				}
				if e := instance.updateSourceConfig(sourcec); e != nil {
					return errors.New("[Config.rpcsdk] write sourceconfig file error:" + e.Error())
				}
				break
			}
			go func() {
				for {
					if e := dao.MongoWatch(selfgroup, selfname, func(opnum int64, config *sconfig.Config) {
						if opnum != instance.opnum {
							//only appconfig can be hot updated
							var e error
							if config == nil {
								e = instance.updateAppConfig("")
							} else {
								e = instance.updateAppConfig(config.AppConfig)
							}
							if e != nil {
								log.Error("[Config.rpcsdk] hot update write appconfig file error:", e)
							} else {
								instance.opnum = opnum
							}
						}
					}); e != nil {
						log.Error("[Config.rpcsdk] watch mongodb error:", e)
						time.Sleep(time.Second)
					}
				}
			}()
			return nil
		}
	}
	//run loop logic
	for {
		//wait for discovery
		resp, e := client.Sinfo(ctx, &api.Sinforeq{Groupname: selfgroup, Appname: selfname, OpNum: instance.opnum})
		if e != nil {
			log.Error("[Config.rpcsdk] call config server for config data error:", e)
			if e == context.DeadlineExceeded || e == context.Canceled {
				return errors.New("[Config.rpcsdk] call config server for config data failed")
			}
			time.Sleep(time.Millisecond * 50)
			continue
		}
		if e := instance.updateAppConfig(resp.CurAppConfig); e != nil {
			return errors.New("[Config.rpcsdk] write appconfig file error:" + e.Error())
		}
		if e := instance.updateSourceConfig(resp.CurSourceConfig); e != nil {
			return errors.New("[Config.rpcsdk] write sourceconfig file error:" + e.Error())
		}
		instance.opnum = resp.OpNum
		break
	}
	go func() {
		if loopInterval < time.Second {
			//too fast
			//if need fast use watch mode
			loopInterval = time.Second
		}
		tker := time.NewTicker(loopInterval)
		for {
			<-tker.C
			ctx, cancel := context.WithTimeout(context.Background(), loopInterval)
			resp, e := client.Sinfo(ctx, &api.Sinforeq{Groupname: selfgroup, Appname: selfname, OpNum: instance.opnum})
			if e != nil {
				log.Error("[Config.rpcsdk] hot update call config server for config data error:", e)
			} else if resp.OpNum != instance.opnum {
				//only appconfig can be hot updated
				if e := instance.updateAppConfig(resp.CurAppConfig); e == nil {
					log.Error("[Config.rpcsdk] hot update write appconfig file error:", e)
				} else {
					instance.opnum = resp.OpNum
				}
			}
			cancel()
			for len(tker.C) > 0 {
				<-tker.C
			}
		}
	}()
	return nil
}

func newmongo(username, passwd, replicaset string, addrs []string) (*mongo.Client, error) {
	op := &options.ClientOptions{}
	if username != "" && passwd != "" {
		op = op.SetAuth(options.Credential{Username: username, Password: passwd})
	}
	if replicaset != "" {
		op.SetReplicaSet(replicaset)
	}
	op = op.SetHosts(addrs)
	op = op.SetConnectTimeout(time.Second)
	op = op.SetCompressors([]string{"zstd"}).SetZstdLevel(3)
	op = op.SetMaxConnIdleTime(time.Minute)
	op = op.SetMaxPoolSize(5)
	op = op.SetSocketTimeout(time.Second)
	op = op.SetHeartbeatInterval(time.Second * 5)
	//default:secondary is preferred to be selected,if there is no secondary,primary will be selected
	op = op.SetReadPreference(readpref.SecondaryPreferred())
	//watch must set readconcern to majority
	op = op.SetReadConcern(readconcern.Majority())
	db, e := mongo.Connect(nil, op)
	if e != nil {
		return nil, e
	}
	return db, nil
}

func (s *sdk) updateAppConfig(appconfig string) error {
	if appconfig == "" {
		appconfig = "{}"
	}
	if len(appconfig) < 2 || appconfig[0] != '{' || appconfig[len(appconfig)-1] != '}' || !json.Valid(common.Str2byte(appconfig)) {
		return errors.New("[Config.sdk.updateAppConfig] data format error")
	}
	appfile, e := os.OpenFile(s.path+"/AppConfig_tmp.json", os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if e != nil {
		log.Error("[Config.sdk.updateAppConfig] open file error:", e)
		return e
	}

	_, e = appfile.Write(common.Str2byte(appconfig))
	if e != nil {
		log.Error("[Config.sdk.updateAppConfig] write file error:", e)
		return e
	}
	if e = appfile.Sync(); e != nil {
		log.Error("[Config.sdk.updateAppConfig] sync to disk error:", e)
		return e
	}
	if e = appfile.Close(); e != nil {
		log.Error("[Config.sdk.updateAppConfig] close file error:", e)
		return e
	}
	if e = os.Rename(s.path+"/AppConfig_tmp.json", s.path+"/AppConfig.json"); e != nil {
		log.Error("[Config.sdk.updateAppConfig] rename error:", e)
		return e
	}
	return nil
}
func (s *sdk) updateSourceConfig(sourceconfig string) error {
	if sourceconfig == "" {
		sourceconfig = "{}"
	}
	if len(sourceconfig) < 2 || sourceconfig[0] != '{' || sourceconfig[len(sourceconfig)-1] != '}' || !json.Valid(common.Str2byte(sourceconfig)) {
		return errors.New("[Config.sdk.updateSourceConfig] data format error")
	}
	sourcefile, e := os.OpenFile(s.path+"/SourceConfig_tmp.json", os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if e != nil {
		log.Error("[Config.sdk.updateSourceConfig] open file error:", e)
		return e
	}
	_, e = sourcefile.Write(common.Str2byte(sourceconfig))
	if e != nil {
		log.Error("[Config.sdk.updateSourceConfig] write file error:", e)
		return e
	}
	if e = sourcefile.Sync(); e != nil {
		log.Error("[Config.sdk.updateSourceConfig] sync to disk error:", e)
		return e
	}
	if e = sourcefile.Close(); e != nil {
		log.Error("[Config.sdk.updateSourceConfig] close file error:", e)
		return e
	}
	if e = os.Rename(s.path+"/SourceConfig_tmp.json", s.path+"/SourceConfig.json"); e != nil {
		log.Error("[Config.sdk.updateSourceConfig] rename error:", e)
		return e
	}
	return nil
}
