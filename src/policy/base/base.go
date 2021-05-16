package base

import (
	"context"
	"fmt"
	"github.com/shopspring/decimal"
	"github.com/timerzz/go-quant/src/account"
	"github.com/timerzz/go-quant/src/cfg"
	"github.com/timerzz/go-quant/src/pusher"
	"io"
	"os"
	"path"
	"time"

	"github.com/adshao/go-binance/v2"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
)

const (
	MINQTY   = 10 //最小交易个数
	BaseCoin = "USDT"
)

type BasePolicy struct {
	Cfg     cfg.PolicysCfg
	Symbol  string
	MaxQty  decimal.Decimal //
	cli     *binance.Client
	Log     *logrus.Logger
	logPath string
	Pusher  pusher.Pusher
	ActCtl  *account.BinanceController
	BaseAvg decimal.Decimal //成本均价
	buyAvg  decimal.Decimal //一次购买的均价
	Profit  decimal.Decimal //预计累计的盈亏
	ch      chan account.BalanceChangeEvent
}

func NewBasePolicy(cfg cfg.PolicysCfg, logPath string, cli *binance.Client, pusher pusher.Pusher, actCtl *account.BinanceController) *BasePolicy {
	var p = &BasePolicy{
		Symbol:  cfg.Coin + BaseCoin,
		cli:     cli,
		logPath: logPath,
		Pusher:  pusher,
		Cfg:     cfg,
		ActCtl:  actCtl,
		ch:      make(chan account.BalanceChangeEvent, 5),
	}
	p.ActCtl.AddListener(cfg.Coin, p.ch)
	p.Run()
	p.InitMaxQty()
	return p
}

func (b *BasePolicy) Run() {
	//初始化日志
	b.Log = logrus.New()
	p := path.Join(b.logPath, b.Cfg.Name, "log")
	w, _ := rotatelogs.New(
		p+".%Y%m%d%H",
		rotatelogs.WithLinkName(p),
		rotatelogs.WithMaxAge(time.Duration(24*7)*time.Hour),
		rotatelogs.WithRotationTime(time.Duration(24)*time.Hour),
	)
	fileAndStdoutWriter := io.MultiWriter(w, os.Stdout)
	b.Log.SetOutput(fileAndStdoutWriter)

	//更新maxQty
	go b.listen()
}

func (b *BasePolicy) Buy(qty decimal.Decimal) error {
	if b.MaxQty.LessThanOrEqual(decimal.NewFromInt(MINQTY)) {
		b.Log.Infof("qty no enough")
		return nil
	}
	return b.TrackOrder(func() (*binance.CreateOrderResponse, error) {
		order, err := b.cli.NewCreateOrderService().Symbol(b.Symbol).
			Side(binance.SideTypeBuy).Type(binance.OrderTypeMarket).QuoteOrderQty(qty.StringFixedBank(3)).
			Do(context.Background())
		if err == nil && order.Status == binance.OrderStatusTypeFilled {
			price, avg, q := b.calAvg(order.Fills)
			b.buyAvg = avg //乐观的认为上一个buyAvg已经被消费了
			msg := fmt.Sprintf("以%s的均价，买入%s个%s，总价%s %s。", avg.StringFixedBank(5), q.StringFixedBank(3), b.Cfg.Coin, price.StringFixedBank(5), BaseCoin)
			b.Log.Info(msg)
			b.Pusher.Push(msg)
		}
		return order, err
	})
}

func (b *BasePolicy) Sell(qty decimal.Decimal) error {
	return b.TrackOrder(func() (*binance.CreateOrderResponse, error) {
		b.Log.Infof("%s try sell , qty %s", b.Symbol, qty)
		order, err := b.cli.NewCreateOrderService().Symbol(b.Symbol).
			Side(binance.SideTypeSell).Type(binance.OrderTypeMarket).Quantity(qty.StringFixedBank(1)).Do(context.Background())
		if err == nil && order.Status == binance.OrderStatusTypeFilled {
			_, avg, q := b.calAvg(order.Fills)
			profit := avg.Sub(b.BaseAvg).Mul(q)
			b.Profit = b.Profit.Add(profit)
			msg := fmt.Sprintf("以%s的均价卖出%s个%s，预计盈亏%s, 累计盈亏%s",
				avg.StringFixedBank(3),
				q.StringFixedBank(1),
				b.Cfg.Coin, profit.StringFixedBank(3),
				b.Profit.StringFixedBank(3),
			)
			b.Log.Infof(msg)
			b.Pusher.Push(msg)
		}
		return order, err
	})
}

func (b *BasePolicy) SellAll() error {
	qty := b.ActCtl.Get(b.Cfg.Coin)
	if decimal.Zero.LessThan(qty) {
		return b.Sell(qty)
	}
	return nil
}

func (b *BasePolicy) TrackOrder(fun func() (*binance.CreateOrderResponse, error)) error {
	order, err := fun()
	if err != nil {
		return err
	}
	if order.Status != binance.OrderStatusTypeFilled {
		go func() {
			<-time.NewTimer(time.Minute * 1).C
			o, err := b.cli.NewGetOrderService().Symbol(b.Symbol).
				OrderID(order.OrderID).Do(context.Background())
			if err != nil {
				b.Log.Errorf("获取订单%v信息失败：%v", order.OrderID, err)
				return
			}
			if o.Status == binance.OrderStatusTypeNew ||
				o.Status == binance.OrderStatusTypePartiallyFilled ||
				o.Status == binance.OrderStatusTypeExpired {
				_, err = b.cli.NewCancelOrderService().Symbol(b.Symbol).
					OrderID(o.OrderID).Do(context.Background())
				if err != nil {
					b.Log.Errorf("取消订单%v失败：%v", o.OrderID, err)
				} else {
					b.Log.Infof("取消订单%d", o.OrderID)
				}
			}
		}()
	}
	return err
}

func (b *BasePolicy) InitMaxQty() {
	free := b.ActCtl.Get(BaseCoin)
	if b.Cfg.USDT < 1 {
		if decimal.Zero.LessThan(free) {
			b.MaxQty = free.Mul(decimal.NewFromFloat(b.Cfg.USDT))
		} else {
			b.MaxQty = decimal.Zero
		}
	} else {
		if decimal.NewFromFloat(b.Cfg.USDT).LessThan(free) {
			b.MaxQty = decimal.NewFromFloat(b.Cfg.USDT)
		} else {
			b.MaxQty = free
		}
	}
	b.Log.Infof("%s maxQty %s", b.Symbol, b.MaxQty.String())
}

//计算总的成本均价
func (b *BasePolicy) calBaseAvg(event account.BalanceChangeEvent) {
	//新增才重新计算
	if event.Old.LessThan(event.New) {
		for b.buyAvg.LessThanOrEqual(decimal.Zero) {
			time.Sleep(time.Millisecond)
		}
		if b.BaseAvg.Equal(decimal.Zero) {
			b.BaseAvg = b.buyAvg
			b.Pusher.Push(fmt.Sprintf("当前%s的成本价为%s, 共%s个", b.Cfg.Coin, b.BaseAvg.StringFixedBank(5), event.New.StringFixedBank(3)))
			return
		}
		b.BaseAvg = b.BaseAvg.Mul(event.Old).Add(b.buyAvg.Mul(event.New.Sub(event.Old))).Div(event.New)
		b.buyAvg = decimal.Zero
		b.Pusher.Push(fmt.Sprintf("当前%s的成本价为%s, 共%s个", b.Cfg.Coin, b.BaseAvg.StringFixedBank(5), event.New.StringFixedBank(3)))
	}
}

// CalBuyQty 计算购买数量
func (b *BasePolicy) CalBuyQty() decimal.Decimal {
	if (1 < b.Cfg.Buy && b.Cfg.Buy < MINQTY) || b.MaxQty.LessThan(decimal.NewFromInt(MINQTY)) {
		return decimal.Zero
	}
	if 1 < b.Cfg.Buy {
		return decimal.NewFromFloat(b.Cfg.Buy)
	}
	targetNum := b.MaxQty.Mul(decimal.NewFromFloat(b.Cfg.Buy))
	if targetNum.LessThan(decimal.NewFromInt(MINQTY)) {
		return decimal.NewFromInt(MINQTY)
	}
	return targetNum
}

func (b *BasePolicy) String() string {
	return fmt.Sprintf("%s-%s", b.Cfg.Name, b.Symbol)
}

func (b *BasePolicy) listen() {
	for {
		event := <-b.ch
		b.InitMaxQty()
		b.Log.Infof("update event %v", event)
		b.calBaseAvg(event) //更新下成本
	}
}

//计算均价
func (b *BasePolicy) calAvg(fills []*binance.Fill) (prices, avg, qty decimal.Decimal) {
	prices, qty = decimal.Zero, decimal.Zero
	for _, fill := range fills {
		p, _ := decimal.NewFromString(fill.Price)
		q, _ := decimal.NewFromString(fill.Quantity)
		prices, qty = prices.Add(p.Mul(q)), qty.Add(q)
	}
	avg = prices.Div(qty)
	return
}
