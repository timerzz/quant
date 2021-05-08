package policy

import (
	"github.com/adshao/go-binance/v2"
	"github.com/timerzz/go-quant/src/cfg"
	"github.com/timerzz/go-quant/src/policy/base"
	buyLowSellHigh "github.com/timerzz/go-quant/src/policy/buy-low-sell-high"
	"github.com/timerzz/go-quant/src/pusher"
)

func NewPolicy(cfg cfg.PolicysCfg, log string, cli *binance.Client, pusher pusher.Pusher) Policy {
	switch cfg.Type {
	case base.BuyLowSellHigh:
		return buyLowSellHigh.NewPolicy(cfg, log, cli, pusher)
	}
	return nil
}
