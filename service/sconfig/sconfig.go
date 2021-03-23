package sconfig

import (
	"context"

	"config/api"
	"config/config"
	sconfigdao "config/dao/sconfig"

	"github.com/chenjie199234/Corelib/log"
	//"github.com/chenjie199234/Corelib/rpc"
	//"github.com/chenjie199234/Corelib/web"
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
	if sum == nil && conf == nil {
		//nothing changed
		return &api.Sinforesp{OpNum: in.OpNum}, nil
	}
	temp := make([]string, 0, len(sum.AllIds)+1)
	for _, v := range sum.AllIds {
		temp = append(temp, v.Hex())
	}
	return &api.Sinforesp{CurId: sum.CurId.Hex(), AllIds: temp, OpNum: sum.OpNum, CurAppConfig: conf.AppConfig, CurSourceConfig: conf.SourceConfig}, nil
}

//set one specific app's config
func (s *Service) Sset(ctx context.Context, in *api.Ssetreq) (*api.Ssetresp, error) {
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

//Stop -
func (s *Service) Stop() {

}
