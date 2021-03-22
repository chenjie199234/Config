package xweb

import (
	"time"

	"config/api"
	"config/config"
	"config/service"

	"github.com/chenjie199234/Corelib/log"
	"github.com/chenjie199234/Corelib/web"
	"github.com/chenjie199234/Corelib/web/mids"
)

var s *web.WebServer

//StartWebServer -
func StartWebServer() {
	c := config.GetWebConfig()
	webc := &web.Config{
		Timeout:            time.Duration(c.WebTimeout),
		StaticFileRootPath: c.WebStaticFile,
		MaxHeader:          1024,
		ReadBuffer:         1024,
		WriteBuffer:        1024,
	}
	if c.WebCors != nil {
		webc.Cors = &web.CorsConfig{
			AllowedOrigin:    c.WebCors.CorsOrigin,
			AllowedHeader:    c.WebCors.CorsHeader,
			ExposeHeader:     c.WebCors.CorsExpose,
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
	if e = api.RegisterCconfigWebServer(s, service.SvcCconfig, mids.AllMids()); e != nil {
		log.Error("[xweb] register handlers error:", e)
		return
	}
	//example
	//if e = api.RegisterExampleWebServer(s, service.SvcExample, mids.AllMids()); e != nil {
	//log.Error("[xweb] register handlers error:", e)
	//return
	//}

	if e = s.StartWebServer(":8000", c.WebCertFile, c.WebKeyFile); e != nil {
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
