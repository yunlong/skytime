package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sync"

	"github.com/golang/glog"
)

/////////////// hot configuration reload for skytime  /////////////////
type Config struct {
	Mode      string
	CacheSize int
}

var (
	config     *Config
	configLock = new(sync.RWMutex)
)

func loadConfig(fail bool) {
	file, err := ioutil.ReadFile("config.json")
	if err != nil {
		glog.Infof("open config: %v", err)
		if fail {
			os.Exit(1)
		}
	}

	temp := new(Config)
	if err = json.Unmarshal(file, temp); err != nil {
		glog.Infof("parse config:%v ", err)
		if fail {
			os.Exit(1)
		}
	}
	configLock.Lock()
	config = temp
	configLock.Unlock()
}

func GetConfig() *Config {
	configLock.RLock()
	defer configLock.RUnlock()
	return config
}
