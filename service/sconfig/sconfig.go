package sconfig

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/chenjie199234/Config/api"
	"github.com/chenjie199234/Config/config"
	sconfigdao "github.com/chenjie199234/Config/dao/sconfig"
	"github.com/chenjie199234/Corelib/log"
	"github.com/chenjie199234/Corelib/util/common"
	"go.mongodb.org/mongo-driver/mongo"
)

//Service subservice for sconfig business
type Service struct {
	sconfigDao *sconfigdao.Dao
}

//Start -
func Start() *Service {
	return &Service{
		sconfigDao: sconfigdao.NewDao(nil, nil, config.GetMongo("config_mongo")),
	}
}

//one specific app's current info
func (s *Service) Sinfo(ctx context.Context, in *api.Sinforeq) (*api.Sinforesp, error) {
	sum, conf, e := s.sconfigDao.MongoGetInfo(ctx, in.Groupname, in.Appname, in.OpNum)
	if e != nil {
		log.Error("[sconfig.Info] error:", e)
		return nil, e
	}
	if sum == nil {
		//not exist
		return &api.Sinforesp{}, nil
	} else if conf == nil {
		//nothing changed
		return &api.Sinforesp{OpNum: in.OpNum}, nil
	}
	temp := make([]string, 0, len(sum.AllIds))
	for _, v := range sum.AllIds {
		temp = append(temp, v.Hex())
	}
	return &api.Sinforesp{CurId: sum.CurId.Hex(), AllIds: temp, OpNum: sum.OpNum, CurAppConfig: conf.AppConfig, CurSourceConfig: conf.SourceConfig}, nil
}

//set one specific app's config
func (s *Service) Sset(ctx context.Context, in *api.Ssetreq) (*api.Ssetresp, error) {
	if in.AppConfig == "" {
		in.AppConfig = "{}"
	}
	if len(in.AppConfig) < 2 || in.AppConfig[0] != '{' || in.AppConfig[len(in.AppConfig)-1] != '}' || !json.Valid(common.Str2byte(in.AppConfig)) {
		return nil, errors.New("app_config must in json object format")
	}
	if in.SourceConfig == "" {
		in.SourceConfig = "{}"
	}
	if len(in.SourceConfig) < 2 || in.SourceConfig[0] != '{' || in.SourceConfig[len(in.SourceConfig)-1] != '}' || !json.Valid(common.Str2byte(in.SourceConfig)) {
		return nil, errors.New("source_config must in json object format")
	}
	e := s.sconfigDao.MongoSetConfig(ctx, in.Groupname, in.Appname, in.AppConfig, in.SourceConfig)
	if e != nil {
		log.Error("[sconfig.Set] error:", e)
		return nil, e
	}
	return &api.Ssetresp{}, nil
}

//rollback one specific app's config
func (s *Service) Srollback(ctx context.Context, in *api.Srollbackreq) (*api.Srollbackresp, error) {
	e := s.sconfigDao.MongoRollbackConfig(ctx, in.Groupname, in.Appname, in.Id)
	if e != nil {
		log.Error("[sconfig.Rollback] error:", e)
		return nil, e
	}
	return &api.Srollbackresp{}, nil
}

//get one specific app's config
func (s *Service) Sget(ctx context.Context, in *api.Sgetreq) (*api.Sgetresp, error) {
	conf, e := s.sconfigDao.MongoGetConfig(ctx, in.Groupname, in.Appname, in.Id)
	if e != nil {
		log.Error("[sconfig.Get] error:", e)
		if e == mongo.ErrNoDocuments {
			return &api.Sgetresp{}, nil
		}
		return nil, e
	}
	return &api.Sgetresp{AppConfig: conf.AppConfig, SourceConfig: conf.SourceConfig}, nil
}

//get all groups
func (s *Service) Sgroups(ctx context.Context, in *api.Sgroupsreq) (*api.Sgroupsresp, error) {
	groups, e := s.sconfigDao.MongoGetGroups(ctx)
	if e != nil {
		log.Error("[sconfig.Groups] error:", e)
		return nil, e
	}
	return &api.Sgroupsresp{Groups: groups}, nil
}

//get all apps
func (s *Service) Sapps(ctx context.Context, in *api.Sappsreq) (*api.Sappsresp, error) {
	apps, e := s.sconfigDao.MongoGetApps(ctx, in.Groupname)
	if e != nil {
		log.Error("[sconfig.Apps] error:", e)
		return nil, e
	}
	return &api.Sappsresp{Apps: apps}, nil
}

//set watch addr
func (s *Service) Ssetwatchaddr(ctx context.Context, in *api.Ssetwatchaddrreq) (*api.Ssetwatchaddrresp, error) {
	if in.Username == "" || in.Passwd == "" || len(in.Addrs) == 0 || in.ReplicaSetName == "" {
		if e := s.sconfigDao.MongoDelWatchAddr(ctx); e != nil {
			log.Error("[sconfig.Ssetwatchaddr] error:", e)
			return nil, e
		}
		return &api.Ssetwatchaddrresp{}, nil
	} else {
		if e := s.sconfigDao.MongoSetWatchAddr(ctx, &sconfigdao.WatchAddr{
			Username:       in.Username,
			Passwd:         in.Passwd,
			Addrs:          in.Addrs,
			ReplicaSetName: in.ReplicaSetName,
		}); e != nil {
			log.Error("[sconfig.Ssetwatchaddr] error:", e)
			return nil, e
		}
		return &api.Ssetwatchaddrresp{}, nil
	}
}

//get watch addr
func (s *Service) Sgetwatchaddr(ctx context.Context, in *api.Sgetwatchaddrreq) (*api.Sgetwatchaddrresp, error) {
	wa, e := s.sconfigDao.MongoGetWatchAddr(ctx)
	if e != nil {
		log.Error("[sconfig.Sgetwatchaddr] error:", e)
		if e == mongo.ErrNoDocuments {
			return &api.Sgetwatchaddrresp{}, nil
		}
		return nil, e
	}
	return &api.Sgetwatchaddrresp{
		Username:       wa.Username,
		Passwd:         wa.Passwd,
		Addrs:          wa.Addrs,
		ReplicaSetName: wa.ReplicaSetName,
	}, nil
}

//Stop -
func (s *Service) Stop() {

}
