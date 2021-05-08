### quant

一个golang实现的量化交易个人项目。也就图一乐。真赚钱还得买彩票。

### 计划实现交易策略
- [x] 追涨杀跌
- [ ] 底分型抄底
- [ ] 趋势买卖

### 交易所支持
- [x] 币安
- [ ] 火币

### 通知推送
- [x] IOS (Bark)
- [ ] 安卓
- [ ] windows

### Run
- 构建
  ```shell
  go build -o quant src/main.go
  chmod +x quant 
  ```
- 运行
  ```shell
    ./quant -f cfg.yaml
  ```
### 配置文件
```yaml
#币安的交易密钥
binanceInfo:
  apiKey: xxx
  secretKey: xxx
#交易策略
policys:
- id: bnb     #id随便起，只要别重复
  type: 0     # type暂时都为0，0表示追涨杀跌
  name: bnb追涨杀跌   #策略名随便起
  coin: BNB       #交易的币种，必须大写。
  usdt: 0.3      #交易的本金。如果小于1，那么认为是比例，如果大于1，那么就认为是数额。
  sell: true    # 是否进行卖出
  buy: 0.2       #买入时，占本金的比例，如果大于1，就是固定值
  buyTiger: 0.012  #买入的触发涨幅
  kline: 1m          # k线类型 参数有：1m 3m 5m 15m 30m 1h等
  interval: 60     #交易间隔，两次交易间的最小间隔，单位是秒
  profit: 0.2     #止盈点
  loss: 0.2      #止损点
#日志配置
logCfg:
  runLog: /var/log/go-quant/run/run.log      #运行日志
  exchangeLog: /var/log/go-quant/exchange    #交易日志
#手机推送配置
pushCfg:
  bark:
    url: 170.106.176.58:8080
    token: 7ZRfSEd5eW3tQCffvxktF8    # bark的token

```


