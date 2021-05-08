package api

import (
	"github.com/adshao/go-binance/v2"
	"github.com/timerzz/go-quant/src/cfg"
)

func Init(info cfg.BinanceInfo) *binance.Client {
	return binance.NewClient(info.ApiKey, info.SecretKey)
}
