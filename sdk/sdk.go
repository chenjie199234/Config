package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
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
	path string
}

var instance *sdk

func NewWebSdk(path, selfgroup, selfname string) error {
	if !atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&instance)), nil, unsafe.Pointer(&sdk{
		path: path,
	})) {
		return nil
	}
	webc := &web.ClientConfig{
		DiscoverInterval: time.Second * 10,
		DiscoverFunction: func(group, name string) (map[string]*web.RegisterData, error) {
			result := make(map[string]*web.RegisterData)
			addrs, e := net.LookupHost(name + "-service-headless" + "." + group)
			if e != nil {
				log.Error("[Config.websdk] get:", name+"-service-headless", "addrs error:", e)
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
		},
	}
	client, e := api.NewSconfigWebClient(webc, selfgroup, selfname, api.Group, api.Name)
	if e != nil {
		log.Error("[Config.websdk] new config client error:", e)
		return e
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancel()
	var resp *api.SwatchaddrResp
	for {
		resp, e = client.Swatchaddr(ctx, &api.SwatchaddrReq{}, nil)
		if e != nil {
			log.Error("[Config.websdk] call config server for watch addr error:", e)
			if e == context.DeadlineExceeded || e == context.Canceled {
				return errors.New("[Config.websdk] get watch addrs failed")
			}
			time.Sleep(time.Millisecond * 25)
			continue
		}
		break
	}
	if len(resp.Addrs) == 0 {
		log.Error("[Config.websdk] init watch error: config server doesn't support watch")
		return errors.New("[Config.websdk] watch addrs emppty")
	} else if db, e := newmongo(ctx, resp.Username, resp.Passwd, resp.ReplicaSetName, resp.Addrs); e != nil {
		log.Error("[Config.websdk] init mongodb error:", e)
		return errors.New("[Config.websdk] init mongodb failed")
	} else {
		//run watch logic
		dao := sconfig.NewDao(nil, nil, db)
		initch := make(chan error, 1)
		notice := func(e error) {
			select {
			case initch <- e:
			default:
			}
		}
		go func() {
			for {
				if e := dao.MongoWatch(selfgroup, selfname, func(config *sconfig.Config) {
					if e := instance.updateAppConfig(config.AppConfig); e != nil {
						log.Error("[Config.websdk] write appconfig file error:", e)
						notice(e)
					}
					if e := instance.updateSourceConfig(config.SourceConfig); e != nil {
						log.Error("[Config.websdk] write sourceconfig file error:", e)
						notice(e)
					}
					notice(nil)
				}); e != nil {
					log.Error("[Config.websdk] watch mongodb error:", e)
					notice(e)
					time.Sleep(time.Millisecond * 500)
				}
			}
		}()
		return <-initch
	}
}

func NewRpcSdk(path, selfgroup, selfname string) error {
	if !atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&instance)), nil, unsafe.Pointer(&sdk{
		path: path,
	})) {
		return nil
	}
	rpcc := &rpc.ClientConfig{
		DiscoverInterval: time.Second * 10,
		DiscoverFunction: func(group, name string) (map[string]*rpc.RegisterData, error) {
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
		},
	}
	client, e := api.NewSconfigRpcClient(rpcc, selfgroup, selfname, api.Group, api.Name)
	if e != nil {
		log.Error("[Config.rpcsdk] new config client error:", e)
		return e
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancel()
	var resp *api.SwatchaddrResp
	for {
		resp, e = client.Swatchaddr(ctx, &api.SwatchaddrReq{})
		if e != nil {
			log.Error("[Config.rpcsdk] call config server for watch addr error:", e)
			if e == context.DeadlineExceeded || e == context.Canceled {
				return errors.New("[Config.rpcsdk] get watch addr failed")
			}
			time.Sleep(time.Millisecond * 25)
			continue
		}
		break
	}
	if len(resp.Addrs) == 0 {
		log.Error("[Config.rpcsdk] init watch error: config server doesn't support watch")
		return errors.New("[Config.rpcsdk] config server watch addrs emppty")
	} else if db, e := newmongo(ctx, resp.Username, resp.Passwd, resp.ReplicaSetName, resp.Addrs); e != nil {
		log.Error("[Config.rpcsdk] init watch error:", e)
		return errors.New("[Config.rpcsdk] init watch failed")
	} else {
		//run watch logic
		dao := sconfig.NewDao(nil, nil, db)
		initch := make(chan error, 1)
		notice := func(e error) {
			select {
			case initch <- e:
			default:
			}
		}
		go func() {
			for {
				if e := dao.MongoWatch(selfgroup, selfname, func(config *sconfig.Config) {
					if e := instance.updateAppConfig(config.AppConfig); e != nil {
						log.Error("[Config.rpcsdk] write appconfig file error:", e)
						notice(e)
					}
					if e := instance.updateAppConfig(config.AppConfig); e != nil {
						log.Error("[Config.rpcsdk] write sourceconfig file error:", e)
						notice(e)
					}
					notice(nil)
				}); e != nil {
					log.Error("[Config.rpcsdk] watch mongodb error:", e)
					notice(e)
					time.Sleep(time.Millisecond * 500)
				}
			}
		}()
		return <-initch
	}
}

func newmongo(ctx context.Context, username, passwd, replicaset string, addrs []string) (*mongo.Client, error) {
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
	db, e := mongo.Connect(ctx, op)
	if e != nil {
		return nil, e
	}
	if e = db.Ping(ctx, nil); e != nil {
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
		log.Error("[Config.sdk.updateAppConfig] open tmp file error:", e)
		return e
	}

	_, e = appfile.Write(common.Str2byte(appconfig))
	if e != nil {
		log.Error("[Config.sdk.updateAppConfig] write temp file error:", e)
		return e
	}
	if e = appfile.Sync(); e != nil {
		log.Error("[Config.sdk.updateAppConfig] sync tmp file to disk error:", e)
		return e
	}
	if e = appfile.Close(); e != nil {
		log.Error("[Config.sdk.updateAppConfig] close tmp file error:", e)
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
		log.Error("[Config.sdk.updateSourceConfig] open tmp file error:", e)
		return e
	}
	_, e = sourcefile.Write(common.Str2byte(sourceconfig))
	if e != nil {
		log.Error("[Config.sdk.updateSourceConfig] write tmp file error:", e)
		return e
	}
	if e = sourcefile.Sync(); e != nil {
		log.Error("[Config.sdk.updateSourceConfig] sync tmp file to disk error:", e)
		return e
	}
	if e = sourcefile.Close(); e != nil {
		log.Error("[Config.sdk.updateSourceConfig] close tmp file error:", e)
		return e
	}
	if e = os.Rename(s.path+"/SourceConfig_tmp.json", s.path+"/SourceConfig.json"); e != nil {
		log.Error("[Config.sdk.updateSourceConfig] rename error:", e)
		return e
	}
	return nil
}
