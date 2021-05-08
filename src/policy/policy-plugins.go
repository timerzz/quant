package policy

import (
	"errors"
	"github.com/adshao/go-binance/v2"
	"github.com/sirupsen/logrus"
	"github.com/timerzz/go-quant/src/account"
	"github.com/timerzz/go-quant/src/cfg"
	"github.com/timerzz/go-quant/src/pusher"
)

type Policy interface {
	Run()
	String() string
}

type Controller struct {
	plugins []Policy
}

func NewController(cli *binance.Client, cfg *cfg.Config, pusher pusher.Pusher, actCtl *account.BinanceController) *Controller {
	plugins := make([]Policy, 0, len(cfg.Policys))
	for _, c := range cfg.Policys {
		if plugin := NewPolicy(c, cfg.LogCfg.ExchangeLog, cli, pusher, actCtl); plugin != nil {
			plugins = append(plugins, plugin)
		}
	}
	return &Controller{
		plugins: plugins,
	}
}

func (c *Controller) Run() error {
	logrus.Infof("quant run start")
	var ch = make(chan struct{}, len(c.plugins))
	for _, plugin := range c.plugins {
		go func(p Policy) {
			logrus.Infof("policy %v run start...", p.String())
			p.Run()
			ch <- struct{}{}
		}(plugin)
	}
	for i := 0; i < len(c.plugins); i++ {
		<-ch
	}
	close(ch)
	return errors.New("run 退出")
}
