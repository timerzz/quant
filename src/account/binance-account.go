package account

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/adshao/go-binance/v2"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/timerzz/go-quant/src/pusher"
	"strings"
	"sync"
	"time"
)

const (
	EventTypeOutboundAccountPosition = "outboundAccountPosition"
)

/************
接收ws传递来的json数据
*************/

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

/************
BinanceController 用于记录币安的更新
**************/

type BinanceController struct {
	cli       *binance.Client
	balances  map[string]*BalanceController
	listenKey string //币安ws需求的listenKey
	pusher    pusher.Pusher
	cancel    context.CancelFunc
	lock      sync.Mutex
}

func NewBinanceController(cli *binance.Client, pusher pusher.Pusher) *BinanceController {
	return &BinanceController{cli: cli, pusher: pusher}
}

func (b *BinanceController) init() error {
	res, err := b.cli.NewGetAccountService().Do(context.Background())
	if err != nil {
		return err
	}
	var balans = make(map[string]*BalanceController, 5)
	for _, balance := range res.Balances {
		free, er := decimal.NewFromString(balance.Free)
		if er != nil {
			return er
		}
		if decimal.NewFromInt(0).LessThan(free) {
			balans[balance.Asset] = &BalanceController{
				Coin:     balance.Asset,
				lasTime:  0,
				qty:      free,
				channels: make([]chan BalanceChangeEvent, 0),
			}
		}
	}
	b.balances = balans
	return nil
}

func (b *BinanceController) Get(assert string) decimal.Decimal {
	b.lock.Lock()
	defer b.lock.Unlock()
	balance, ok := b.balances[assert]
	if !ok {
		return decimal.NewFromInt(0)
	}
	return balance.qty
}
func (b *BinanceController) Run() error {
	if err := b.init(); err != nil {
		return err
	}
	go func() {
		for b.run() != nil {
		}
	}()
	return nil
}
func (b *BinanceController) run() (err error) {
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
		var updateCoins = make([]string, 0, 3)
		for _, balance := range event.Balances {
			free, er := decimal.NewFromString(balance.Free)
			if er != nil {
				logrus.Error("Unmarshall free err:", er)
				continue
			}
			b.updateBalanceController(balance.Asset, free, event.EventTime)
			updateCoins = append(updateCoins, balance.Asset)
			logrus.Infof("user data update %s : %s", balance.Asset, balance.Free)
		}
		b.pusher.Push(fmt.Sprintf("货币：%s 数量更新。", strings.Join(updateCoins, "、")))
	}
}

func (b *BinanceController) updateBalanceController(asset string, qty decimal.Decimal, lastTime uint64) {
	b.lock.Lock()
	defer b.lock.Unlock()
	balance, ok := b.balances[asset]
	if !ok {
		b.balances[asset] = &BalanceController{
			Coin:     asset,
			lasTime:  lastTime,
			qty:      qty,
			channels: make([]chan BalanceChangeEvent, 0),
			lock:     sync.Mutex{},
		}
		return
	}
	balance.Update(qty, lastTime)
}

func (b *BinanceController) AddListener(asset string, ch chan BalanceChangeEvent) {
	b.lock.Lock()
	defer b.lock.Unlock()
	balance, ok := b.balances[asset]
	if !ok {
		b.balances[asset] = &BalanceController{
			Coin:     asset,
			lasTime:  0,
			qty:      decimal.NewFromInt(0),
			channels: []chan BalanceChangeEvent{ch},
			lock:     sync.Mutex{},
		}
		return
	}
	balance.AddListener(ch)
}

func (b *BinanceController) RmListener(asset string, ch chan BalanceChangeEvent) {
	b.lock.Lock()
	defer b.lock.Unlock()
	balance, ok := b.balances[asset]
	if !ok {
		return
	}
	balance.RmListener(ch)
}

// BalanceController 单个货币的控制器
type BalanceController struct {
	Coin     string
	lasTime  uint64                    //上次更新时间
	qty      decimal.Decimal           //货币数量
	channels []chan BalanceChangeEvent //需要主动通知的chan
	lock     sync.Mutex                //增删channel的锁
}

type BalanceChangeEvent struct {
	Old decimal.Decimal
	New decimal.Decimal
}

// Broadcast 通知更新
func (b *BalanceController) Broadcast(change BalanceChangeEvent) {
	for _, c := range b.channels {
		if c != nil {
			c <- change
		}
	}
}

// Update 更新数量
func (b *BalanceController) Update(qty decimal.Decimal, updateTime uint64) {
	if updateTime > b.lasTime {
		old := b.qty
		b.lasTime, b.qty = updateTime, qty
		b.Broadcast(BalanceChangeEvent{New: qty, Old: old})
	}
}

// AddListener 添加监听
func (b *BalanceController) AddListener(ch chan BalanceChangeEvent) {
	b.lock.Lock()
	b.channels = append(b.channels, ch)
	b.lock.Unlock()
}

// RmListener 删除监听
func (b *BalanceController) RmListener(ch chan BalanceChangeEvent) {
	b.lock.Lock()
	defer b.lock.Unlock()
	for i, v := range b.channels {
		if v == ch {
			var chans = b.channels[:i]
			if i != len(b.channels)-1 {
				chans = append(chans, b.channels[i+1:len(b.channels)-1]...)
			}
			b.channels = chans
			close(ch)
			return
		}
	}
}
