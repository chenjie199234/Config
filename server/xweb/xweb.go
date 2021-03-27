package xweb

import (
	"sync"
	"time"

	"github.com/chenjie199234/Config/api"
	"github.com/chenjie199234/Config/config"
	"github.com/chenjie199234/Config/service"
	"github.com/chenjie199234/Corelib/log"
	"github.com/chenjie199234/Corelib/web"
	"github.com/chenjie199234/Corelib/web/mids"
	discoverysdk "github.com/chenjie199234/Discovery/sdk"
)

var s *web.WebServer

//StartWebServer -
func StartWebServer(wg *sync.WaitGroup) {
	c := config.GetWebServerConfig()
	webc := &web.ServerConfig{
		GlobalTimeout:      time.Duration(c.GlobalTimeout),
		IdleTimeout:        time.Duration(c.IdleTimeout),
		HeartProbe:         time.Duration(c.HeartProbe),
		StaticFileRootPath: c.StaticFile,
		MaxHeader:          1024,
		SocketRBuf:         1024,
		SocketWBuf:         1024,
	}
	if c.Cors != nil {
		webc.Cors = &web.CorsConfig{
			AllowedOrigin:    c.Cors.CorsOrigin,
			AllowedHeader:    c.Cors.CorsHeader,
			ExposeHeader:     c.Cors.CorsExpose,
			AllowCredentials: true,
			MaxAge:           24 * time.Hour,
		}
	}
	var e error
	if s, e = web.NewWebServer(webc, api.Group, api.Name); e != nil {
		log.Error("[xweb] new error:", e)
		return
	}

	//this place can register global midwares
	//s.Use(globalmidwares)

	//you just need to register your service here
	if e = api.RegisterStatusWebServer(s, service.SvcStatus, mids.AllMids()); e != nil {
		log.Error("[xweb] register handlers error:", e)
		return
	}
	if e = api.RegisterSconfigWebServer(s, service.SvcSconfig, mids.AllMids()); e != nil {
		log.Error("[xweb] register handlers error:", e)
		return
	}
	//example
	//if e = api.RegisterExampleWebServer(s, service.SvcExample, mids.AllMids()); e != nil {
	//log.Error("[xweb] register handlers error:", e)
	//return
	//}

	if config.EC.ServerVerifyDatas != nil {
		if len(c.CertKey) > 0 {
			e = discoverysdk.RegWeb(8000, "http")
		} else {
			e = discoverysdk.RegWeb(8000, "https")
		}
		if e != nil {
			log.Error("[xweb] register web to discovery server error:", e)
			return
		}
	}
	wg.Done()
	if e = s.StartWebServer(":8000", c.CertKey); e != nil {
		log.Error("[xweb] start error:", e)
		return
	}
}

//StopWebServer -
func StopWebServer() {
	if s != nil {
		s.StopWebServer()
	}
}
