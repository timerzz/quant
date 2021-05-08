package account

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/adshao/go-binance/v2"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

const (
	EventTypeOutboundAccountPosition = "outboundAccountPosition"
)

type OutboundAccountPosition struct {
	EventType  string     `json:"e"`
	EventTime  uint64     `json:"E"`
	UpdateUnix uint64     `json:"u"`
	Balances   []*Balance `json:"B"`
}

type Balance struct {
	Asset  string `json:"a"`
	Free   string `json:"f"`
	Locked string `json:"l"`
}

type BinanceController struct {
	lastTime  uint64 //上次更新时间
	cli       *binance.Client
	balances  map[string]decimal.Decimal
	listenKey string
	cancel    context.CancelFunc
	lock      sync.Mutex
}

func NewBinanceController(cli *binance.Client) *BinanceController {
	return &BinanceController{cli: cli, lastTime: uint64(time.Now().Unix())}
}

func (b *BinanceController) init() error {
	res, err := b.cli.NewGetAccountService().Do(context.Background())
	if err != nil {
		return err
	}
	var balans = make(map[string]decimal.Decimal, 5)
	for _, balance := range res.Balances {
		free, er := decimal.NewFromString(balance.Free)
		if er != nil {
			return er
		}
		if decimal.NewFromInt(0).LessThan(free) {
			balans[balance.Asset] = free
		}
	}
	b.balances = balans
	return nil
}

func (b *BinanceController) Get(assert string) (dec decimal.Decimal) {
	var ok bool
	b.lock.Lock()
	defer b.lock.Unlock()
	if dec, ok = b.balances[assert]; !ok {
		dec = decimal.NewFromInt(0)
	}
	return
}
func (b *BinanceController) Run() {
	go func() {
		for b.run() != nil {
		}
	}()
}
func (b *BinanceController) run() (err error) {
	if err = b.init(); err != nil {
		return
	}
	var ctx context.Context
	ctx, b.cancel = context.WithCancel(context.Background())
	defer b.cancel()
	if err = b.keepAlive(ctx); err != nil {
		return
	}

	doneC, _, err := binance.WsUserDataServe(b.listenKey, b.wsHandler, func(err error) {
		logrus.Error(err)
	})
	if err != nil {
		return err
	}
	<-doneC
	return errors.New("binanceController run done")
}

func (b *BinanceController) keepAlive(ctx context.Context) (err error) {
	b.listenKey, err = b.cli.NewStartUserStreamService().Do(context.Background())
	if err != nil {
		return
	}
	logrus.Infof("listenKey:%s", b.listenKey)
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				b.cli.NewCloseUserStreamService().ListenKey(b.listenKey).Do(context.Background())
				return
			case <-ticker.C:
				if err = b.cli.NewKeepaliveUserStreamService().ListenKey(b.listenKey).Do(ctx); err != nil {
					logrus.Error("user data keepalive err: ", err)
				}
			}
		}
	}()
	return
}

func (b *BinanceController) wsHandler(message []byte) {
	if bytes.Contains(message, []byte(EventTypeOutboundAccountPosition)) {
		var event OutboundAccountPosition
		err := json.Unmarshal(message, &event)
		if err != nil {
			logrus.Error("Unmarshall json err:", err)
			return
		}
		//时间更晚，才更新
		if event.EventTime > b.lastTime {
			b.lastTime = event.EventTime
			for _, balance := range event.Balances {
				free, er := decimal.NewFromString(balance.Free)
				if er != nil {
					logrus.Error("Unmarshall free err:", er)
					continue
				}
				b.lock.Lock()
				b.balances[balance.Asset] = free
				b.lock.Unlock()
				logrus.Infof("user data update %s : %s", balance.Asset, balance.Free)
			}
		}
	}
}
