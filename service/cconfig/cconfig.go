package cconfig

import (
	"context"

	"config/api"
	"config/config"
	cconfigdao "config/dao/cconfig"

	"github.com/chenjie199234/Corelib/log"
	//"github.com/chenjie199234/Corelib/rpc"
	//"github.com/chenjie199234/Corelib/web"
)

//Service subservice for cconfig business
type Service struct {
	cconfigDao *cconfigdao.Dao
}

//Start -
func Start() *Service {
	return &Service{
		cconfigDao: cconfigdao.NewDao(nil, nil, config.GetMongo("config_mongo")),
	}
}

//one specific app's current info
func (s *Service) Cinfo(ctx context.Context, in *api.Cinforeq) (*api.Cinforesp, error) {
	curid, data, allids, e := s.cconfigDao.MongoGetInfo(ctx, in.Groupname, in.Appname)
	if e != nil {
		log.Error("[cconfig.Info] error:", e)
		return nil, e
	}
	return &api.Cinforesp{CurId: curid, CurConfig: data, AllIds: allids}, nil
}

//set one specific app's config
func (s *Service) Cset(ctx context.Context, in *api.Csetreq) (*api.Csetresp, error) {
	e := s.cconfigDao.MongoSetConfig(ctx, in.Groupname, in.Appname, in.Config)
	if e != nil {
		log.Error("[cconfig.Set] error:", e)
		return nil, e
	}
	return &api.Csetresp{}, nil
}

//get one specific app's config
func (s *Service) Cget(ctx context.Context, in *api.Cgetreq) (*api.Cgetresp, error) {
	data, e := s.cconfigDao.MongoGetConfig(ctx, in.Groupname, in.Appname, in.Id)
	if e != nil {
		log.Error("[cconfig.Get] error:", e)
		return nil, e
	}
	return &api.Cgetresp{Config: data}, nil
}

//get all groups
func (s *Service) Cgroups(ctx context.Context, in *api.Cgroupsreq) (*api.Cgroupsresp, error) {
	groups, e := s.cconfigDao.MongoGetGroups(ctx)
	if e != nil {
		log.Error("[cconfig.Groups] error:", e)
		return nil, e
	}
	return &api.Cgroupsresp{Groups: groups}, nil
}

//get all apps
func (s *Service) Capps(ctx context.Context, in *api.Cappsreq) (*api.Cappsresp, error) {
	apps, e := s.cconfigDao.MongoGetApps(ctx, in.Groupname)
	if e != nil {
		log.Error("[cconfig.Apps] error:", e)
		return nil, e
	}
	return &api.Cappsresp{Apps: apps}, nil
}

//Stop -
func (s *Service) Stop() {

}
