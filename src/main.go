package main

import (
	"flag"
	"fmt"
	"github.com/timerzz/go-quant/src/api"
	cfg2 "github.com/timerzz/go-quant/src/cfg"
	"github.com/timerzz/go-quant/src/logger"
	"github.com/timerzz/go-quant/src/policy"
	"github.com/timerzz/go-quant/src/pusher"
)

func main() {
	var cfgPath string //配置文件路径
	flag.StringVar(&cfgPath, "f", "cfg.yaml", "配置文件路径,默认 cfg.yaml")
	flag.Parse()

	config, err := cfg2.InitCfg(cfgPath)
	if err != nil {
		fmt.Println("获取配置失败！", err)
		return
	}
	logger.Init(config.LogCfg)
	client := api.Init(config.BinanceInfo)
	bark := pusher.NewBarkPush(config.PushCfg.BarkCfg)
	c := policy.NewController(client, config, bark)
	bark.Push("bark test")
	for c.Run() != nil {

	}
}
