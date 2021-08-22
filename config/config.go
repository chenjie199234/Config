package config

import (
	"encoding/json"
	"os"
	"strconv"

	"github.com/chenjie199234/Corelib/log"
)

//EnvConfig can't hot update,all these data is from system env setting
//nil field means that system env not exist
type EnvConfig struct {
	ServerVerifyDatas []string
	ConfigType        *int
	RunEnv            *string
	DeployEnv         *string
}

//EC -
var EC *EnvConfig

//notice is a sync function
//don't write block logic inside it
func Init(notice func(c *AppConfig)) {
	initenv()
	var path string
	if EC.ConfigType == nil || *EC.ConfigType == 0 {
		path = "./"
	} else {
		path = "./kubeconfig/"
	}
	initsource(path)
	initapp(path, notice)
}

//Close -
func Close() {
	log.Close()
}

func initenv() {
	EC = &EnvConfig{}
	if str, ok := os.LookupEnv("SERVER_VERIFY_DATA"); ok && str != "<SERVER_VERIFY_DATA>" {
		temp := make([]string, 0)
		if str != "" {
			EC.ServerVerifyDatas = temp
		} else if e := json.Unmarshal([]byte(str), &temp); e != nil {
			log.Error("[config.initenv] SERVER_VERIFY_DATA must be json string array like:[\"abc\",\"123\"]")
			Close()
			os.Exit(1)
		}
		EC.ServerVerifyDatas = temp
	} else {
		log.Warning("[config.initenv] missing SERVER_VERIFY_DATA")
	}
	if str, ok := os.LookupEnv("CONFIG_TYPE"); ok && str != "<CONFIG_TYPE>" && str != "" {
		configtype, e := strconv.Atoi(str)
		if e != nil || (configtype != 0 && configtype != 1) {
			log.Error("[config.initenv] CONFIG_TYPE must be number in [0,1]")
			Close()
			os.Exit(1)
		}
		EC.ConfigType = &configtype
	} else {
		log.Warning("[config.initenv] missing CONFIG_TYPE")
	}
	if str, ok := os.LookupEnv("RUN_ENV"); ok && str != "<RUN_ENV>" && str != "" {
		EC.RunEnv = &str
	} else {
		log.Warning("[config.initenv] missing RUN_ENV")
	}
	if str, ok := os.LookupEnv("DEPLOY_ENV"); ok && str != "<DEPLOY_ENV>" && str != "" {
		EC.DeployEnv = &str
	} else {
		log.Warning("[config.initenv] missing DEPLOY_ENV")
	}
}
