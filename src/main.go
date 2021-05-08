package main

import (
	"flag"
	"fmt"
	"github.com/timerzz/go-quant/src/account"
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
	//获取配置
	config, err := cfg2.InitCfg(cfgPath)
	if err != nil {
		fmt.Println("获取配置失败！", err)
		return
	}
	//初始化日志
	logger.Init(config.LogCfg)
	//初始化binanc api
	client := api.Init(config.BinanceInfo)
	//初始化手机推送
	bark := pusher.NewBarkPush(config.PushCfg.BarkCfg)
	//账户信息控制器
	actCtl := account.NewBinanceController(client)
	actCtl.Run()
	//初始化各策略
	c := policy.NewController(client, config, bark, actCtl)
	bark.Push("bark test")
	for c.Run() != nil {

	}
}
