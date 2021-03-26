package service

import (
	"config/dao"
	"config/service/sconfig"
	"config/service/status"
)

//SvcStatus one specify sub service
var SvcStatus *status.Service

//SvcSconfig one specify sub service
var SvcSconfig *sconfig.Service

//StartService start the whole service
func StartService() error {
	if e := dao.NewApi(); e != nil {
		return e
	}
	//start sub service
	SvcStatus = status.Start()
	SvcSconfig = sconfig.Start()
	return nil
}

//StopService stop the whole service
func StopService() {
	//stop sub service
	SvcStatus.Stop()
	SvcSconfig.Stop()
}
