package sconfig

import (
	"context"
	"encoding/json"

	"github.com/chenjie199234/Config/api"
	"github.com/chenjie199234/Config/config"
	sconfigdao "github.com/chenjie199234/Config/dao/sconfig"
	"github.com/chenjie199234/Config/ecode"

	"github.com/chenjie199234/Corelib/log"
	"github.com/chenjie199234/Corelib/util/common"
	"go.mongodb.org/mongo-driver/mongo"
)

//Service subservice for sconfig business
type Service struct {
	mongoname  string
	sconfigDao *sconfigdao.Dao
}

//Start -
func Start() *Service {
	s := &Service{
		mongoname: "config_mongo",
	}
	s.sconfigDao = sconfigdao.NewDao(nil, nil, config.GetMongo(s.mongoname))
	return s
}

//one specific app's current info
func (s *Service) Sinfo(ctx context.Context, in *api.SinfoReq) (*api.SinfoResp, error) {
	sum, conf, e := s.sconfigDao.MongoGetInfo(ctx, in.Groupname, in.Appname)
	if e != nil {
		log.Error("[sconfig.Sinfo] error:", e)
		if e == mongo.ErrNoDocuments {
			return nil, ecode.ErrNotExist
		}
		return nil, ecode.ErrSystem
	}
	return &api.SinfoResp{CurIndex: sum.CurIndex, MaxIndex: sum.MaxIndex, OpNum: sum.OpNum, CurAppConfig: conf.AppConfig, CurSourceConfig: conf.SourceConfig}, nil
}

//set one specific app's config
func (s *Service) Sset(ctx context.Context, in *api.SsetReq) (*api.SsetResp, error) {
	if in.AppConfig == "" {
		in.AppConfig = "{}"
	}
	if len(in.AppConfig) < 2 || in.AppConfig[0] != '{' || in.AppConfig[len(in.AppConfig)-1] != '}' || !json.Valid(common.Str2byte(in.AppConfig)) {
		return nil, ecode.ErrCoinfigFormat
	}
	if in.SourceConfig == "" {
		in.SourceConfig = "{}"
	}
	if len(in.SourceConfig) < 2 || in.SourceConfig[0] != '{' || in.SourceConfig[len(in.SourceConfig)-1] != '}' || !json.Valid(common.Str2byte(in.SourceConfig)) {
		return nil, ecode.ErrCoinfigFormat
	}
	e := s.sconfigDao.MongoSetConfig(ctx, in.Groupname, in.Appname, in.AppConfig, in.SourceConfig)
	if e != nil {
		log.Error("[sconfig.Sset] error:", e)
		return nil, ecode.ErrSystem
	}
	return &api.SsetResp{}, nil
}

//rollback one specific app's config
func (s *Service) Srollback(ctx context.Context, in *api.SrollbackReq) (*api.SrollbackResp, error) {
	status, e := s.sconfigDao.MongoRollbackConfig(ctx, in.Groupname, in.Appname, in.Index)
	if e != nil {
		log.Error("[sconfig.Srollback] error:", e)
		return nil, ecode.ErrSystem
	}
	if !status {
		return nil, ecode.ErrNotExist
	}
	return &api.SrollbackResp{}, nil
}

//get one specific app's config
func (s *Service) Sget(ctx context.Context, in *api.SgetReq) (*api.SgetResp, error) {
	conf, e := s.sconfigDao.MongoGetConfig(ctx, in.Groupname, in.Appname, in.Index)
	if e != nil {
		log.Error("[sconfig.Sget] error:", e)
		if e == mongo.ErrNoDocuments {
			return nil, ecode.ErrNotExist
		}
		return nil, ecode.ErrSystem
	}
	return &api.SgetResp{Index: conf.Index, AppConfig: conf.AppConfig, SourceConfig: conf.SourceConfig}, nil
}

//get all groups
func (s *Service) Sgroups(ctx context.Context, in *api.SgroupsReq) (*api.SgroupsResp, error) {
	groups, e := s.sconfigDao.MongoGetGroups(ctx)
	if e != nil {
		log.Error("[sconfig.Sgroups] error:", e)
		return nil, ecode.ErrSystem
	}
	return &api.SgroupsResp{Groups: groups}, nil
}

//get all apps
func (s *Service) Sapps(ctx context.Context, in *api.SappsReq) (*api.SappsResp, error) {
	apps, e := s.sconfigDao.MongoGetApps(ctx, in.Groupname)
	if e != nil {
		log.Error("[sconfig.Sapps] error:", e)
		return nil, ecode.ErrSystem
	}
	return &api.SappsResp{Apps: apps}, nil
}

//get watch addr
func (s *Service) Swatchaddr(ctx context.Context, in *api.SwatchaddrReq) (*api.SwatchaddrResp, error) {
	c := config.GetMongoC(s.mongoname)
	return &api.SwatchaddrResp{
		Username:       c.Username,
		Passwd:         c.Passwd,
		Addrs:          c.Addrs,
		ReplicaSetName: c.ReplicaSetName,
	}, nil
}

//Stop -
func (s *Service) Stop() {

}
