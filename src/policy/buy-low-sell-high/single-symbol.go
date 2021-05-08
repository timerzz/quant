package buyLowSellHigh

import (
	"fmt"
	"github.com/adshao/go-binance/v2"
	"github.com/shopspring/decimal"
	"github.com/timerzz/go-quant/src/account"
	"github.com/timerzz/go-quant/src/cfg"
	"github.com/timerzz/go-quant/src/policy/base"
	"github.com/timerzz/go-quant/src/pusher"
	"sync/atomic"
	"time"
)

type Policy struct {
	*base.BasePolicy
	lock int32
	avg  decimal.Decimal //均价
}

func NewPolicy(cfg cfg.PolicysCfg, logPath string, cli *binance.Client, pusher pusher.Pusher, actCtl *account.BinanceController) *Policy {
	return &Policy{
		avg:        decimal.NewFromInt(0),
		BasePolicy: base.NewBasePolicy(cfg, logPath, cli, pusher, actCtl),
	}
}

func (p *Policy) Run() {
	doneC, _, err := binance.WsKlineServe(p.Symbol, p.Cfg.Kline, p.wsHandler, func(err error) {
		p.Log.Error(err)
	})
	if err != nil {
		p.Log.Error(err)
		return
	}
	<-doneC
}

func (p *Policy) wsHandler(event *binance.WsKlineEvent) {
	//
	closeP, err := decimal.NewFromString(event.Kline.Close)
	if err != nil {
		p.BasePolicy.Log.Error(err)
		return
	}
	openP, err := decimal.NewFromString(event.Kline.Open)
	if err != nil {
		p.BasePolicy.Log.Error(err)
		return
	}
	p.checkSell(closeP)
	p.checkBuy(closeP, openP)
}

func (p *Policy) checkBuy(closeP, openP decimal.Decimal) {
	//buy
	if decimal.NewFromFloat(p.Cfg.BuyTiger).LessThan(closeP.Div(openP).Sub(decimal.NewFromInt(1))) &&
		atomic.CompareAndSwapInt32(&p.lock, 0, 1) {
		defer func() {
			go time.AfterFunc(time.Second*time.Duration(p.Cfg.Interval), func() {
				atomic.StoreInt32(&p.lock, 0)
			})
		}()
		if qty := p.CalBuyQty(); qty != "" {
			coinQty := p.ActCtl.Get(p.Cfg.Coin) //当前的coin的个数
			if err := p.Buy(qty); err != nil {
				p.Log.Errorf("%s buy err:%v", p.Symbol, err)
				return
			}
			coinValue := coinQty.Mul(p.avg)          //当前的coin价值的usdt=coin的个数乘以当前的均价
			usdtQty, _ := decimal.NewFromString(qty) //用来购买的usdt个数
			for p.ActCtl.Get(p.Cfg.Coin).Equal(coinQty) {
				time.Sleep(time.Millisecond)
			}
			p.avg = coinValue.Add(usdtQty).Div(p.ActCtl.Get(p.Cfg.Coin))
		}
	}
}

func (p *Policy) checkSell(closeP decimal.Decimal) {
	var sellQty = decimal.NewFromInt(0)
	var msg = ""
	if decimal.NewFromInt(0).LessThan(p.avg) {
		//止盈
		var profit = closeP.Div(p.avg).Sub(decimal.NewFromInt(1))
		var profitPoint = decimal.NewFromFloat(p.Cfg.ProfitPoint)
		//止损
		var loss = decimal.NewFromInt(1).Sub(closeP.Div(p.avg))
		var lossPoint = decimal.NewFromFloat(p.Cfg.LossPoint)
		//当前币的个数
		var coinQty = p.ActCtl.Get(p.Cfg.Coin)
		if profitPoint.Mul(decimal.NewFromInt(3)).LessThanOrEqual(profit) {
			sellQty = coinQty.Mul(decimal.NewFromFloat(0.05))
			msg = fmt.Sprintf("达到3倍止盈点，尝试卖出%s个%s, 大约赚", sellQty.StringFixedBank(3), p.Cfg.Coin)
		} else if profitPoint.Mul(decimal.NewFromInt(2)).LessThanOrEqual(profit) {
			sellQty = coinQty.Mul(decimal.NewFromFloat(0.3))
			msg = fmt.Sprintf("达到2倍止盈点，尝试卖出%s个%s, 大约赚", sellQty.StringFixedBank(3), p.Cfg.Coin)
		} else if profitPoint.LessThanOrEqual(profit) {
			sellQty = coinQty.Mul(decimal.NewFromFloat(0.6))
			msg = fmt.Sprintf("达到止盈点，尝试卖出%s个%s, 大约赚", sellQty.StringFixedBank(3), p.Cfg.Coin)
		} else if lossPoint.Mul(decimal.NewFromFloat(1.5)).LessThanOrEqual(loss) {
			msg = fmt.Sprintf("达到1.5倍止损点，尝试卖出%s个%s, 大约亏", sellQty.StringFixedBank(3), p.Cfg.Coin)
			sellQty = coinQty
		} else {
			sellQty = coinQty.Mul(decimal.NewFromFloat(0.8))
			msg = fmt.Sprintf("达到止损点，尝试卖出%s个%s, 大约亏", sellQty.StringFixedBank(3), p.Cfg.Coin)
		}
	}
	if decimal.NewFromInt(0.1).LessThan(sellQty) && atomic.CompareAndSwapInt32(&p.lock, 0, 1) {
		defer func() {
			go time.AfterFunc(time.Second*time.Duration(p.Cfg.Interval), func() {
				atomic.StoreInt32(&p.lock, 0)
			})
		}()
		if err := p.Sell(sellQty.StringFixedBank(3)); err != nil {
			p.Log.Error("sell err", err)
			return
		}
		msg += sellQty.Mul(closeP.Sub(p.avg)).StringFixedBank(3)
		p.Log.Infof(msg)
		p.Pusher.Push(msg)
		p.InitMaxQty()
	}
}
