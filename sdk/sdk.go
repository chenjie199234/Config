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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type sdk struct {
	path      string
	selfgroup string
	selfname  string
	opnum     int64
}

var instance *sdk

//sdktype:
//1-watch,get db addr from config server through web protocol(k8sdns or self's discovry),watch self's config change by watch the db
//2-dbloop,get db addr from config server through web protocol(k8sdns or self's discovry),loop get self's config form db
//3-rpcloop,loop get self's config by call config server through rpc protocol(self's discovry),config server will get data from db and return back
//4-webloop,loop get self's config by call config server through web protocol(k8sdns or self's discovry),config server will get data from db and return back
//k8sdns:
//when sdktype is 1,2,4 sdk will call config server through web protocol
//if k8sdns is true,the call will use k8s's dns
//if k8sdns is false,the call will use self's discovry
//tls:
//when sdktype is 1,2,4 sdk will call config server through web protocol,does this need tls
func NewServerSdk(sdktype int, k8sdns, tls bool, interval time.Duration, path string, selfgroup, selfname string) error {
	if sdktype != 1 && sdktype != 2 && sdktype != 3 && sdktype != 4 {
		return errors.New("[config.sdk] unknown sdk type")
	}
	if !atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&instance)), nil, unsafe.Pointer(&sdk{
		path:  path,
		opnum: 0,
	})) {
		return nil
	}
	if interval == 0 {
		interval = time.Second
	}
	switch sdktype {
	case 1:
		return instance.watch(k8sdns, tls)
	case 2:
		return instance.dbloop(k8sdns, tls, interval)
	case 3:
		return instance.rpcloop(interval)
	case 4:
		return instance.webloop(k8sdns, tls, interval)
	}
	return nil
}
func (s *sdk) dbloop(k8sdns, tls bool, interval time.Duration) error {
	var c api.SconfigWebClient
	var e error
	if !k8sdns {
		c, e = api.NewSconfigWebClient(nil, s.selfgroup, s.selfname)
		if e != nil {
			return e
		}
	} else {
		discover := func(group, name string, client *web.WebClient) {
			all := make(map[string][]string)
			if tls {
				all["https://config.default"] = []string{"k8sdns"}
			} else {
				all["http://config.default"] = []string{"k8sdns"}
			}
			client.UpdateDiscovery(all, nil)
		}
		c, e = api.NewSconfigWebClient(&web.ClientConfig{Discover: discover}, s.selfgroup, s.selfname)
		if e != nil {
			return e
		}
	}
	//init
	var resp *api.Sgetwatchaddrresp
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancel()
	for {
		//discovry and connect may take some time
		//loop and sleep to wait it finish
		time.Sleep(time.Millisecond * 50)
		resp, e = c.Sgetwatchaddr(ctx, &api.Sgetwatchaddrreq{})
		if e != nil {
			log.Error("[config.sdk.dbloop.init] call config server error:", e)
			if e == context.DeadlineExceeded || e == context.Canceled {
				return errors.New("[config.sdk.dbloop.init] call config server failed")
			}
			continue
		}
		break
	}
	if resp.Username == "" || resp.Passwd == "" || len(resp.Addrs) == 0 || resp.ReplicaSetName == "" {
		return errors.New("[config.sdk.dbloop.init] doesn't support")
	}
	op := &options.ClientOptions{}
	op = op.SetAuth(options.Credential{Username: resp.Username, Password: resp.Passwd})
	op.SetReplicaSet(resp.ReplicaSetName)
	op = op.SetHosts(resp.Addrs)
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
		return errors.New("[config.sdk.dbloop.init] create mongodb client error:" + e.Error())
	}
	dao := sconfig.NewDao(nil, nil, db)
	summary, config, e := dao.MongoGetInfo(ctx, s.selfgroup, s.selfname, 0)
	if e == nil {
		if e := s.updateAppConfig(config.AppConfig); e != nil {
			return errors.New("[config.sdk.dbloop.init] write appconfig file error:" + e.Error())
		}
		if e := s.updateSourceConfig(config.SourceConfig); e != nil {
			return errors.New("[config.sdk.dbloop.init] write sourceconfig file error:" + e.Error())
		}
		s.opnum = summary.OpNum
	} else if e == mongo.ErrNoDocuments {
		if e := s.updateAppConfig("{}"); e != nil {
			return errors.New("[config.sdk.dbloop.init] write appconfig file error:" + e.Error())
		}
		if e := s.updateSourceConfig("{}"); e != nil {
			return errors.New("[config.sdk.dbloop.init] write sourceconfig file error:" + e.Error())
		}
		s.opnum = 0
	} else {
		return e
	}
	//init success
	//start hot update
	go func() {
		tker := time.NewTicker(interval)
		for {
			<-tker.C
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			summary, config, e := dao.MongoGetInfo(ctx, s.selfgroup, s.selfname, s.opnum)
			if e == nil {
				if e := s.updateAppConfig(config.AppConfig); e == nil {
					s.opnum = summary.OpNum
				} else {
					log.Error("[config.sdk.dbloop] hot update error:", e)
				}
			} else if e == mongo.ErrNoDocuments {
				if e = s.updateAppConfig("{}"); e == nil {
					s.opnum = 0
				} else {
					log.Error("[config.sdk.dbloop] hot update error:", e)
				}
			} else {
				log.Error("[config.sdk.dbloop] get data from mongodb error:", e)
			}
			cancel()
		}
	}()
	return nil
}
func (s *sdk) rpcloop(interval time.Duration) error {
	c, e := api.NewSconfigRpcClient(nil, s.selfgroup, s.selfname)
	if e != nil {
		return e
	}
	//init
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancel()
	for {
		//discovry and connect may take some time
		//loop and sleep to wait it finish
		time.Sleep(time.Millisecond * 50)
		resp, e := c.Sinfo(ctx, &api.Sinforeq{Groupname: s.selfgroup, Appname: s.selfname, OpNum: s.opnum})
		if e != nil {
			log.Error("[config.sdk.rpcloop.init] call config server error:", e)
			if e == rpc.ERRCTXTIMEOUT || e == rpc.ERRCTXCANCEL {
				return errors.New("[config.sdk.rpcloop.init] call config server failed")
			}
			continue
		}
		if e := s.updateAppConfig(resp.CurAppConfig); e != nil {
			return errors.New("[config.sdk.rpcloop.init] write appconfig file error:" + e.Error())
		}
		if e := s.updateSourceConfig(resp.CurSourceConfig); e != nil {
			return errors.New("[config.sdk.rpcloop.init] write sourceconfig file error:" + e.Error())
		}
		s.opnum = resp.OpNum
		break
	}
	//init success
	//start hot update
	go func() {
		tker := time.NewTicker(interval)
		for {
			<-tker.C
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			//background context will use the client's timeout
			resp, e := c.Sinfo(ctx, &api.Sinforeq{Groupname: s.selfgroup, Appname: s.selfname, OpNum: s.opnum})
			if e != nil {
				log.Error("[config.sdk.rpcloop] call config server error:", e)
			} else if resp.OpNum != s.opnum {
				//only appconfig can be hot updated
				if e := s.updateAppConfig(resp.CurAppConfig); e == nil {
					s.opnum = resp.OpNum
				} else {
					log.Error("[config.sdk.rpcloop] hot update error:", e)
				}
			}
			cancel()
		}
	}()
	return nil
}
func (s *sdk) webloop(k8sdns, tls bool, interval time.Duration) error {
	var c api.SconfigWebClient
	var e error
	if !k8sdns {
		c, e = api.NewSconfigWebClient(nil, s.selfgroup, s.selfname)
		if e != nil {
			return e
		}
	} else {
		discover := func(group, name string, client *web.WebClient) {
			all := make(map[string][]string)
			if tls {
				all["https://config.default"] = []string{"k8sdns"}
			} else {
				all["http://config.default"] = []string{"k8sdns"}
			}
			client.UpdateDiscovery(all, nil)
		}
		c, e = api.NewSconfigWebClient(&web.ClientConfig{Discover: discover}, s.selfgroup, s.selfname)
		if e != nil {
			return e
		}
	}
	//init
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancel()
	for {
		//discovry and connect may take some time
		//loop and sleep to wait it finish
		time.Sleep(time.Millisecond * 50)
		resp, e := c.Sinfo(ctx, &api.Sinforeq{Groupname: s.selfgroup, Appname: s.selfname, OpNum: s.opnum})
		if e != nil {
			log.Error("[config.sdk.webloop.init] call config server error:", e)
			if e == context.DeadlineExceeded || e == context.Canceled {
				return errors.New("[config.sdk.webloop.init] call config server failed")
			}
			continue
		}
		if e := s.updateAppConfig(resp.CurAppConfig); e != nil {
			return errors.New("[config.sdk.webloop.init] write appconfig file error:" + e.Error())
		}
		if e := s.updateSourceConfig(resp.CurSourceConfig); e != nil {
			return errors.New("[config.sdk.webloop.init] write sourceconfig file error:" + e.Error())
		}
		s.opnum = resp.OpNum
		break
	}
	//init success
	//start hot update
	go func() {
		tker := time.NewTicker(interval)
		for {
			<-tker.C
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			//background context will use the client's timeout
			resp, e := c.Sinfo(ctx, &api.Sinforeq{Groupname: s.selfgroup, Appname: s.selfname, OpNum: s.opnum})
			if e != nil {
				log.Error("[config.sdk.webloop] call config server error:", e)
			} else if resp.OpNum != s.opnum {
				//only appconfig can be hot updated
				if e := s.updateAppConfig(resp.CurAppConfig); e == nil {
					s.opnum = resp.OpNum
				} else {
					log.Error("[config.sdk.webloop] hot update error:", e)
				}
			}
			cancel()
		}
	}()
	return nil
}
func (s *sdk) watch(k8sdns, tls bool) error {
	var c api.SconfigWebClient
	var e error
	if !k8sdns {
		c, e = api.NewSconfigWebClient(nil, s.selfgroup, s.selfname)
		if e != nil {
			return e
		}
	} else {
		discover := func(group, name string, client *web.WebClient) {
			all := make(map[string][]string)
			if tls {
				all["https://config.default"] = []string{"k8sdns"}
			} else {
				all["http://config.default"] = []string{"k8sdns"}
			}
			client.UpdateDiscovery(all, nil)
		}
		c, e = api.NewSconfigWebClient(&web.ClientConfig{Discover: discover}, s.selfgroup, s.selfname)
		if e != nil {
			return e
		}
	}
	//init
	var resp *api.Sgetwatchaddrresp
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancel()
	for {
		//discovry and connect may take some time
		//loop and sleep to wait it finish
		time.Sleep(time.Millisecond * 50)
		resp, e = c.Sgetwatchaddr(ctx, &api.Sgetwatchaddrreq{})
		if e != nil {
			log.Error("[config.sdk.watch.init] call config server error:", e)
			if e == context.DeadlineExceeded || e == context.Canceled {
				return errors.New("[config.sdk.watch.init] call config server failed")
			}
			continue
		}
		break
	}
	if resp.Username == "" || resp.Passwd == "" || len(resp.Addrs) == 0 || resp.ReplicaSetName == "" {
		return errors.New("[config.sdk.watch.init] doesn't support")
	}
	op := &options.ClientOptions{}
	op = op.SetAuth(options.Credential{Username: resp.Username, Password: resp.Passwd})
	op.SetReplicaSet(resp.ReplicaSetName)
	op = op.SetHosts(resp.Addrs)
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
		return errors.New("[config.sdk.watch.init] create mongodb client error:" + e.Error())
	}
	dao := sconfig.NewDao(nil, nil, db)
	initch := make(chan struct{}, 1)
	//init success
	//start hot update
	go func() {
		for {
			if e := dao.MongoWatch(s.selfgroup, s.selfname, func(opnum int64, config *sconfig.Config) {
				select {
				case initch <- struct{}{}:
				default:
				}
				if opnum == s.opnum {
					return
				}
				if opnum == 0 || config == nil {
					if e := s.updateAppConfig("{}"); e == nil {
						s.opnum = opnum
					} else {
						log.Error("[config.sdk.watch] hot update error:", e)
					}
				} else if e := s.updateAppConfig(config.AppConfig); e != nil {
					//only appconfig can be hot updated
					s.opnum = opnum
				} else {
					log.Error("[config.sdk.watch] hot update error:", e)
				}
			}); e != nil {
				log.Error("[config.sdk.watch] watch mongodb error:", e)
			}
		}
	}()
	select {
	case <-initch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
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
