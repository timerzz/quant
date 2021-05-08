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
}

func NewBasePolicy(cfg cfg.PolicysCfg, logPath string, cli *binance.Client, pusher pusher.Pusher, actCtl *account.BinanceController) *BasePolicy {
	var p = &BasePolicy{
		Symbol:  cfg.Coin + BaseCoin,
		cli:     cli,
		logPath: logPath,
		Pusher:  pusher,
		Cfg:     cfg,
		ActCtl:  actCtl,
	}
	p.Run()
	p.InitMaxQty()
	return p
}

func (b *BasePolicy) Run() {
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
}

func (b *BasePolicy) Buy(qty string) error {
	if b.MaxQty.LessThanOrEqual(decimal.NewFromInt(10)) {
		b.Log.Infof("qty no enough")
		return nil
	}
	b.Pusher.Push(fmt.Sprintf("尝试买入价值%s USDT的%s ", qty, b.Cfg.Coin))
	return b.TrackOrder(func() (*binance.CreateOrderResponse, error) {
		b.Log.Infof("%s buy , qty %s", b.Symbol, qty)
		order, err := b.cli.NewCreateOrderService().Symbol(b.Symbol).
			Side(binance.SideTypeBuy).Type(binance.OrderTypeMarket).QuoteOrderQty(qty).
			Do(context.Background())
		if err == nil {
			deciQty, err := decimal.NewFromString(qty)
			if err != nil {
				b.Log.Error(err)
			}
			b.MaxQty = b.MaxQty.Sub(deciQty)
			if order.Status == binance.OrderStatusTypeFilled {
				b.Log.Infof("%s buy sucess, %v", b.Symbol, order)
			}
		}
		return order, err
	})
}

func (b *BasePolicy) Sell(qty string) error {
	//b.Pusher.Push(fmt.Sprintf("尝试卖出%s 个%s ", qty, b.Cfg.Coin))
	return b.TrackOrder(func() (*binance.CreateOrderResponse, error) {
		b.Log.Infof("%s sell , qty %s", b.Symbol, qty)
		order, err := b.cli.NewCreateOrderService().Symbol(b.Symbol).
			Side(binance.SideTypeSell).Type(binance.OrderTypeMarket).Quantity(qty).Do(context.Background())
		if err == nil && order.Status == binance.OrderStatusTypeFilled {
			b.Log.Infof("%s 卖出 %s 个", b.Symbol, qty)
		}
		return order, err
	})
}

func (b *BasePolicy) SellAll() error {
	qty := b.ActCtl.Get(b.Cfg.Coin)
	if decimal.NewFromInt(0).LessThan(qty) {
		return b.Sell(qty.StringFixedBank(3))
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
		if decimal.NewFromInt(0).LessThan(free) {
			b.MaxQty = free.Mul(decimal.NewFromFloat(b.Cfg.USDT))
		} else {
			b.MaxQty = decimal.NewFromInt(0)
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

//计算购买数量
func (b *BasePolicy) CalBuyQty() string {
	if (1 < b.Cfg.Buy && b.Cfg.Buy < MINQTY) || b.MaxQty.LessThan(decimal.NewFromInt(MINQTY)) {
		return ""
	}
	if 1 < b.Cfg.Buy {
		return decimal.NewFromFloat(b.Cfg.Buy).StringFixedBank(3)
	}
	targetNum := b.MaxQty.Mul(decimal.NewFromFloat(b.Cfg.Buy))
	if targetNum.LessThan(decimal.NewFromInt(MINQTY)) {
		return decimal.NewFromInt(MINQTY).StringFixedBank(3)
	}
	return targetNum.StringFixedBank(3)
}

func (b *BasePolicy) String() string {
	return fmt.Sprintf("%s-%s", b.Cfg.Name, b.Symbol)
}
