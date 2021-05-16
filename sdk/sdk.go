package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/chenjie199234/Config/dao/sconfig"
	"github.com/chenjie199234/Config/sdk/api"
	"github.com/chenjie199234/Corelib/log"
	"github.com/chenjie199234/Corelib/rpc"
	"github.com/chenjie199234/Corelib/util/common"
	"github.com/chenjie199234/Corelib/web"
	discoverysdk "github.com/chenjie199234/Discovery/sdk"
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

func NewWebSdk(path, selfgroup, selfname string, watch bool, loopInterval time.Duration, kubernetesdns bool) error {
	if !atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&instance)), nil, unsafe.Pointer(&sdk{
		path:  path,
		opnum: 0,
	})) {
		return nil
	}
	webc := &web.ClientConfig{
		Discover: discoverysdk.DefaultWebDiscover,
	}
	if kubernetesdns {
		webc.Discover = func(group, name string, client *web.WebClient) {
			serveraddr := make(map[string][]string)
			serveraddr[name+"-service."+group] = []string{"kubernetesdns"}
			client.UpdateDiscovery(serveraddr, nil)
		}
	}
	client, e := api.NewSconfigWebClient(webc, selfgroup, selfname)
	if e != nil {
		log.Error("[config.sdk] new config client error:", e)
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
				log.Error("[config.sdk] call config server for watch addr error:", e)
				if e == context.DeadlineExceeded || e == context.Canceled {
					return errors.New("[config.sdk] call config server for watch addr failed")
				}
				continue
			}
			break
		}
		if len(resp.Addrs) == 0 {
			log.Warning("[config.sdk] init watch error: config server doesn't support watch")
			log.Warning("[config.sdk] fallback to loop")
		} else if db, e := newmongo(resp.Username, resp.Passwd, resp.ReplicaSetName, resp.Addrs); e != nil {
			log.Warning("[config.sdk] init watch error:", e)
			log.Warning("[config.sdk] fallback to loop")
		} else {
			//run watch logic
			dao := sconfig.NewDao(nil, nil, db)
			for {
				sum, conf, e := dao.MongoGetInfo(ctx, selfgroup, selfname, 0)
				if e != nil {
					log.Error("[config.sdk] get config data from db error:", e)
					if e == context.DeadlineExceeded || e == context.Canceled {
						return errors.New("[config.sdk] get config data from db failed")
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
					return errors.New("[config.sdk] write appconfig file error:" + e.Error())
				}
				if e := instance.updateSourceConfig(sourcec); e != nil {
					return errors.New("[config.sdk] write sourceconfig file error:" + e.Error())
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
								log.Error("[config.sdk] hot update write appconfig file error:", e)
							} else {
								instance.opnum = opnum
							}
						}
					}); e != nil {
						log.Error("[config.sdk.watch] watch mongodb error:", e)
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
			log.Error("[config.sdk] call config server for config data error:", e)
			if e == context.DeadlineExceeded || e == context.Canceled {
				return errors.New("[config.sdk] call config server for config data failed")
			}
			continue
		}
		if e := instance.updateAppConfig(resp.CurAppConfig); e != nil {
			return errors.New("[config.sdk] write appconfig file error:" + e.Error())
		}
		if e := instance.updateSourceConfig(resp.CurSourceConfig); e != nil {
			return errors.New("[config.sdk] write sourceconfig file error:" + e.Error())
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
				log.Error("[config.sdk] hot update call config server for config data error:", e)
			} else if resp.OpNum != instance.opnum {
				//only appconfig can be hot updated
				if e := instance.updateAppConfig(resp.CurAppConfig); e != nil {
					log.Error("[config.sdk] hot update write appconfig file error:", e)
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
func NewRpcSdk(path, selfgroup, selfname string, watch bool, loopInterval time.Duration) error {
	if !atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&instance)), nil, unsafe.Pointer(&sdk{
		path:  path,
		opnum: 0,
	})) {
		return nil
	}
	rpcc := &rpc.ClientConfig{
		Discover: discoverysdk.DefaultRpcDiscover,
	}
	client, e := api.NewSconfigRpcClient(rpcc, selfgroup, selfname)
	if e != nil {
		log.Error("[config.sdk] new config client error:", e)
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
				log.Error("[config.sdk] call config server for watch addr error:", e)
				if e == context.DeadlineExceeded || e == context.Canceled {
					return errors.New("[config.sdk] call config server for watch addr failed")
				}
				continue
			}
			break
		}
		if len(resp.Addrs) == 0 {
			log.Warning("[config.sdk] init watch error: config server doesn't support watch")
			log.Warning("[config.sdk] fallback to loop")
		} else if db, e := newmongo(resp.Username, resp.Passwd, resp.ReplicaSetName, resp.Addrs); e != nil {
			log.Warning("[config.sdk] init watch error:", e)
			log.Warning("[config.sdk] fallback to loop")
		} else {
			//run watch logic
			dao := sconfig.NewDao(nil, nil, db)
			for {
				sum, conf, e := dao.MongoGetInfo(ctx, selfgroup, selfname, 0)
				if e != nil {
					log.Error("[config.sdk] get config data from db error:", e)
					if e == context.DeadlineExceeded || e == context.Canceled {
						return errors.New("[config.sdk] get config data from db failed")
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
					return errors.New("[config.sdk] write appconfig file error:" + e.Error())
				}
				if e := instance.updateSourceConfig(sourcec); e != nil {
					return errors.New("[config.sdk] write sourceconfig file error:" + e.Error())
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
								log.Error("[config.sdk] hot update write appconfig file error:", e)
							} else {
								instance.opnum = opnum
							}
						}
					}); e != nil {
						log.Error("[config.sdk.watch] watch mongodb error:", e)
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
			log.Error("[config.sdk] call config server for config data error:", e)
			if e == context.DeadlineExceeded || e == context.Canceled {
				return errors.New("[config.sdk] call config server for config data failed")
			}
			time.Sleep(time.Millisecond * 50)
			continue
		}
		if e := instance.updateAppConfig(resp.CurAppConfig); e != nil {
			return errors.New("[config.sdk] write appconfig file error:" + e.Error())
		}
		if e := instance.updateSourceConfig(resp.CurSourceConfig); e != nil {
			return errors.New("[config.sdk] write sourceconfig file error:" + e.Error())
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
				log.Error("[config.sdk] hot update call config server for config data error:", e)
			} else if resp.OpNum != instance.opnum {
				//only appconfig can be hot updated
				if e := instance.updateAppConfig(resp.CurAppConfig); e != nil {
					log.Error("[config.sdk] hot update write appconfig file error:", e)
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
		return errors.New("[config.sdk.updateAppConfig] data format error")
	}
	appfile, e := os.OpenFile(s.path+"/AppConfig_tmp.json", os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if e != nil {
		log.Error("[config.sdk.updateAppConfig] open file error:", e)
		return e
	}

	n, e := appfile.WriteString(appconfig)
	if e != nil {
		log.Error("[config.sdk.updateAppConfig] write file error:", e)
		return e
	}
	if n != len(appconfig) {
		log.Error("[config.sdk.updateAppConfig] write file error: short write")
		return e
	}
	if e = appfile.Sync(); e != nil {
		log.Error("[config.sdk.updateAppConfig] sync to disk error:", e)
		return e
	}
	if e = appfile.Close(); e != nil {
		log.Error("[config.sdk.updateAppConfig] close file error:", e)
		return e
	}
	if e = os.Rename(s.path+"/AppConfig_tmp.json", s.path+"/AppConfig.json"); e != nil {
		log.Error("[config.sdk.updateAppConfig] rename error:", e)
		return e
	}
	return nil
}
func (s *sdk) updateSourceConfig(sourceconfig string) error {
	if sourceconfig == "" {
		sourceconfig = "{}"
	}
	if len(sourceconfig) < 2 || sourceconfig[0] != '{' || sourceconfig[len(sourceconfig)-1] != '}' || !json.Valid(common.Str2byte(sourceconfig)) {
		return errors.New("[config.sdk.updateSourceConfig] data format error")
	}
	sourcefile, e := os.OpenFile(s.path+"/SourceConfig_tmp.json", os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if e != nil {
		log.Error("[config.sdk.updateSourceConfig] open file error:", e)
		return e
	}
	n, e := sourcefile.WriteString(sourceconfig)
	if e != nil {
		log.Error("[config.sdk.updateSourceConfig] write file error:", e)
		return e
	}
	if n != len(sourceconfig) {
		log.Error("[config.sdk.updateSourceConfig] write file error: short write")
		return e
	}
	if e = sourcefile.Sync(); e != nil {
		log.Error("[config.sdk.updateSourceConfig] sync to disk error:", e)
		return e
	}
	if e = sourcefile.Close(); e != nil {
		log.Error("[config.sdk.updateSourceConfig] close file error:", e)
		return e
	}
	if e = os.Rename(s.path+"/SourceConfig_tmp.json", s.path+"/SourceConfig.json"); e != nil {
		log.Error("[config.sdk.updateSourceConfig] rename error:", e)
		return e
	}
	return nil
}
