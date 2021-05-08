package api

import (
	"github.com/adshao/go-binance/v2"
	"github.com/timerzz/go-quant/src/cfg"
)

//var (
//	apiKey    = "cuDX6HT17Cwegs9qBayrSvv5Uz7hVbBkn5RQ48Ivdgj3nOFtqGFtoTDNJHopjfVy"
//	secretKey = "gUhiQz8Fg2nu4s8QzhniNQdqM9cdZbjydG0ropS7fmzElc6MulIjH4ANOfqitUrE"
//	Client    *binance.Client
//)

func Init(info cfg.BinanceInfo) *binance.Client {
	return binance.NewClient(info.ApiKey, info.SecretKey)
}
