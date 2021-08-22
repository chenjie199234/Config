package xweb

import (
	"time"

	"github.com/chenjie199234/Config/api"
	"github.com/chenjie199234/Config/config"
	"github.com/chenjie199234/Config/service"

	"github.com/chenjie199234/Corelib/log"
	"github.com/chenjie199234/Corelib/web"
	"github.com/chenjie199234/Corelib/web/mids"
)

var s *web.WebServer

//StartWebServer -
func StartWebServer() {
	c := config.GetWebServerConfig()
	webc := &web.ServerConfig{
		WaitCloseRefresh:   true,
		GlobalTimeout:      time.Duration(c.GlobalTimeout),
		IdleTimeout:        time.Duration(c.IdleTimeout),
		HeartProbe:         time.Duration(c.HeartProbe),
		StaticFileRootPath: c.StaticFilePath,
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

	if e = s.StartWebServer(":8000", nil); e != nil {
		if e != web.ErrServerClosed {
			log.Error("[xweb] start error:", e)
		} else {
			log.Info("[xweb] server closed")
		}
		return
	}
}

//StopWebServer -
func StopWebServer() {
	if s != nil {
		s.StopWebServer()
	}
}
