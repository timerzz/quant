package cfg

type Config struct {
	BinanceInfo BinanceInfo  `yaml:"binanceInfo"`
	Policys     []PolicysCfg `yaml:"policys"`
	LogCfg      LogCfg       `yaml:"logCfg"`
	PushCfg     PushCfg      `yaml:"pushCfg"`
}

//币安信息
type BinanceInfo struct {
	ApiKey    string `yaml:"apiKey"`
	SecretKey string `yaml:"secretKey"`
}

type PolicysCfg struct {
	ID          string  `yaml:"id"`
	Type        int     `yaml:"type"`
	Name        string  `yaml:"name"`
	Coin        string  `yaml:"coin"`
	USDT        float64 `yaml:"usdt"` //可以是比例也可以是具体的数字
	Sell        bool    `yaml:"sell"`
	Buy         float64 `yaml:"buy"`
	BuyTiger    float64 `yaml:"buyTiger"` //触发买入的涨幅
	Kline       string  `yaml:"kline"`
	Interval    int     `yaml:"interval"`
	ProfitPoint float64 `yaml:"profit"`
	LossPoint   float64 `yaml:"loss"`
}

type LogCfg struct {
	RunLog      string `yaml:"runLog"`
	ExchangeLog string `yaml:"exchangeLog"`
}

type PushCfg struct {
	BarkCfg BarkCfg `yaml:"bark"`
}

type BarkCfg struct {
	Url   string `yaml:"url"`
	Token string `yaml:"token"`
}
