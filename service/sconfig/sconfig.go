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
	curid, data, allids, opnum, e := s.sconfigDao.MongoGetInfo(ctx, in.Groupname, in.Appname)
	if e != nil {
		log.Error("[sconfig.Info] error:", e)
		return nil, e
	}
	if opnum == in.OpNum && opnum != 0 {
		return &api.Sinforesp{OpNum: opnum}, nil
	}
	return &api.Sinforesp{CurId: curid, CurConfig: data, AllIds: allids, OpNum: opnum}, nil
}

//set one specific app's config
func (s *Service) Sset(ctx context.Context, in *api.Ssetreq) (*api.Ssetresp, error) {
	e := s.sconfigDao.MongoSetConfig(ctx, in.Groupname, in.Appname, in.Config)
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
	data, e := s.sconfigDao.MongoGetConfig(ctx, in.Groupname, in.Appname, in.Id)
	if e != nil {
		log.Error("[sconfig.Get] error:", e)
		return nil, e
	}
	return &api.Sgetresp{Config: data}, nil
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
