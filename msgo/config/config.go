package config

import (
	"flag"
	"github.com/BurntSushi/toml"
	msLog "github.com/bulon99/msgo/log"
	"os"
)

var Conf = &MsConfig{
	logger:   msLog.Default(),
	Log:      make(map[string]any),
	Template: make(map[string]any),
	Db:       make(map[string]any),
	Pool:     make(map[string]any),
}

type MsConfig struct {
	logger   *msLog.Logger
	Log      map[string]any
	Template map[string]any
	Db       map[string]any
	Pool     map[string]any
}

func init() {
	loadToml()
}

func loadToml() {
	configFile := flag.String("conf", "conf/app.toml", "app config file")
	flag.Parse()
	if _, err := os.Stat(*configFile); err != nil {
		Conf.logger.Info("conf/app.toml file not load，because not exist")
		return
	}
	_, err := toml.DecodeFile(*configFile, Conf) //将配置文件内容加载的Conf
	if err != nil {
		Conf.logger.Info("conf/app.toml decode fail check format")
		return
	}
}
