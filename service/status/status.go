package status

import (
	"context"
	"time"

	//"config/config"
	"github.com/chenjie199234/Config/api"
	statusdao "github.com/chenjie199234/Config/dao/status"
	//"github.com/chenjie199234/Config/ecode"
	//"github.com/chenjie199234/Corelib/log"
	//"github.com/chenjie199234/Corelib/rpc"
	//"github.com/chenjie199234/Corelib/web"
)

//Service subservice for status business
type Service struct {
	statusDao *statusdao.Dao
}

//Start -
func Start() *Service {
	return &Service{
		//statusDao: statusdao.NewDao(config.GetSql("status_sql"), config.GetRedis("status_redis"), config.GetMongo("status_mongo")),
		statusDao: statusdao.NewDao(nil, nil, nil),
	}
}

func (s *Service) Ping(ctx context.Context, in *api.Pingreq) (*api.Pingresp, error) {
	//if _, ok := ctx.(*rpc.Context); ok {
	//        log.Info("this is a rpc call")
	//}
	//if _, ok := ctx.(*web.Context); ok {
	//        log.Info("this is a web call")
	//}
	return &api.Pingresp{ClientTimestamp: in.Timestamp, ServerTimestamp: time.Now().UnixNano()}, nil
}

//Stop -
func (s *Service) Stop() {

}
