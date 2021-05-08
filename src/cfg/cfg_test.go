package cfg

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"testing"
)

func TestConfig(t *testing.T) {
	var cfg = Config{
		BinanceInfo: BinanceInfo{
			ApiKey:    "cuDX6HT17Cwegs9qBayrSvv5Uz7hVbBkn5RQ48Ivdgj3nOFtqGFtoTDNJHopjfVy",
			SecretKey: "gUhiQz8Fg2nu4s8QzhniNQdqM9cdZbjydG0ropS7fmzElc6MulIjH4ANOfqitUrE",
		},
		Policys: []PolicysCfg{{
			ID:       "bnb",
			Type:     0,
			Name:     "bnb追涨杀跌",
			Coin:     "BNB",
			USDT:     0.3,
			Sell:     false,
			BuyRatio: 0.2,
			BuyTiger: 0.012,
		}},
		LogCfg: LogCfg{
			RunLog:      "/var/log/go-quant/run/run.log",
			ExchangeLog: "/var/log/go-quant/exchange/",
		},
	}
	out, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Error(err)
	}
	if err = ioutil.WriteFile("quant-cfg.yaml", out, 755); err != nil {
		t.Error(err)
	}
}

func TestUnmarsh(t *testing.T) {
	in, err := ioutil.ReadFile("cfg.yaml")
	if err != nil {
		t.Error(err)
	}
	var cfg Config
	if err = yaml.Unmarshal(in, &cfg); err != nil {
		t.Error(err)
	}
	t.Log(cfg)
}
