package cfg

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

func InitCfg(path string) (cfg *Config, err error) {
	var in []byte
	if in, err = ioutil.ReadFile(path); err != nil {
		return
	}
	err = yaml.Unmarshal(in, &cfg)
	return
}
