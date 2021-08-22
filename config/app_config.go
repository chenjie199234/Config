package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/chenjie199234/Corelib/log"
	"github.com/fsnotify/fsnotify"
)

//AppConfig can hot update
//this is the config used for this app
type AppConfig struct {
	//add your config here
}

//AC -
var AC *AppConfig

var watcher *fsnotify.Watcher

func initapp(path string, notice func(*AppConfig)) {
	data, e := os.ReadFile(path + "AppConfig.json")
	if e != nil {
		log.Error("[config.initapp] read config file error:", e)
		Close()
		os.Exit(1)
	}
	AC = &AppConfig{}
	if e = json.Unmarshal(data, AC); e != nil {
		log.Error("[config.initapp] config file format error:", e)
		Close()
		os.Exit(1)
	}
	if notice != nil {
		notice(AC)
	}
	watcher, e = fsnotify.NewWatcher()
	if e != nil {
		log.Error("[config.initapp] create watcher for hot update error:", e)
		Close()
		os.Exit(1)
	}
	if e = watcher.Add(path); e != nil {
		log.Error("[config.initapp] create watcher for hot update error:", e)
		Close()
		os.Exit(1)
	}
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if EC.ConfigType == nil || *EC.ConfigType == 0 || *EC.ConfigType == 2 {
					if filepath.Base(event.Name) != "AppConfig.json" || (event.Op&fsnotify.Create == 0 && event.Op&fsnotify.Write == 0) {
						continue
					}
				} else {
					//k8s mount volume is different
					if filepath.Base(event.Name) != "..data" || event.Op&fsnotify.Create == 0 {
						continue
					}
				}
				data, e := os.ReadFile(path + "AppConfig.json")
				if e != nil {
					log.Error("[config.initapp] hot update read config file error:", e)
					continue
				}
				c := &AppConfig{}
				if e = json.Unmarshal(data, c); e != nil {
					log.Error("[config.initapp] hot update config file format error:", e)
					continue
				}
				AC = c
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Error("[config.initapp] hot update watcher error:", err)
			}
		}
	}()
}