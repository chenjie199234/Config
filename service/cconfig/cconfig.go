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
	sum, conf, e := s.cconfigDao.MongoGetInfo(ctx, in.Groupname, in.Appname, in.OpNum)
	if e != nil {
		log.Error("[cconfig.Info] error:", e)
		return nil, e
	}
	temp := make([]string, 0, len(sum.AllIds)+1)
	for _, v := range sum.AllIds {
		temp = append(temp, v.Hex())
	}
	return &api.Cinforesp{CurId: sum.CurId.Hex(), AllIds: temp, OpNum: sum.OpNum, CurAppConfig: conf.AppConfig, CurSourceConfig: conf.SourceConfig}, nil
}

//set one specific app's config
func (s *Service) Cset(ctx context.Context, in *api.Csetreq) (*api.Csetresp, error) {
	e := s.cconfigDao.MongoSetConfig(ctx, in.Groupname, in.Appname, in.AppConfig, in.SourceConfig)
	if e != nil {
		log.Error("[cconfig.Set] error:", e)
		return nil, e
	}
	return &api.Csetresp{}, nil
}

//rollback one specific app's config
func (s *Service) Crollback(ctx context.Context, in *api.Crollbackreq) (*api.Crollbackresp, error) {
	e := s.cconfigDao.MongoRollbackConfig(ctx, in.Groupname, in.Appname, in.Id)
	if e != nil {
		log.Error("[cconfig.Rollback] error:", e)
		return nil, e
	}
	return &api.Crollbackresp{}, nil
}

//get one specific app's config
func (s *Service) Cget(ctx context.Context, in *api.Cgetreq) (*api.Cgetresp, error) {
	conf, e := s.cconfigDao.MongoGetConfig(ctx, in.Groupname, in.Appname, in.Id)
	if e != nil {
		log.Error("[cconfig.Get] error:", e)
		return nil, e
	}
	return &api.Cgetresp{AppConfig: conf.AppConfig, SourceConfig: conf.SourceConfig}, nil
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
