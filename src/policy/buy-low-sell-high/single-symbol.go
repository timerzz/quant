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
	buyLock  int32
	sellLock int32
}

func NewPolicy(cfg cfg.PolicysCfg, logPath string, cli *binance.Client, pusher pusher.Pusher, actCtl *account.BinanceController) *Policy {
	return &Policy{
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
		atomic.CompareAndSwapInt32(&p.buyLock, 0, 1) {
		defer func() {
			go time.AfterFunc(time.Second*time.Duration(p.Cfg.BuyInterval), func() {
				atomic.StoreInt32(&p.buyLock, 0)
			})
		}()
		if qty := p.CalBuyQty(); decimal.Zero.LessThan(qty) {
			if err := p.Buy(qty); err != nil {
				p.Log.Errorf("%s buy err:%v", p.Symbol, err)
				return
			}
		}
	}
}

func (p *Policy) checkSell(closeP decimal.Decimal) {
	var sellQty = decimal.Zero
	var msg = ""
	if decimal.Zero.LessThan(p.BaseAvg) {
		//止盈
		var profit = closeP.Div(p.BaseAvg).Sub(decimal.NewFromInt(1))
		var profitPoint = decimal.NewFromFloat(p.Cfg.ProfitPoint)
		//止损
		var lossPoint = decimal.NewFromFloat(-p.Cfg.LossPoint)
		//当前币的个数
		var coinQty = p.ActCtl.Get(p.Cfg.Coin)
		if profitPoint.Mul(decimal.NewFromInt(3)).LessThanOrEqual(profit) {
			sellQty = coinQty.Mul(decimal.NewFromFloat(0.95))
			msg = fmt.Sprintf("达到3倍止盈点，尝试卖出%s个%s", sellQty.StringFixedBank(3), p.Cfg.Coin)
		} else if profitPoint.Mul(decimal.NewFromInt(2)).LessThanOrEqual(profit) {
			sellQty = coinQty.Mul(decimal.NewFromFloat(0.7))
			msg = fmt.Sprintf("达到2倍止盈点，尝试卖出%s个%s,", sellQty.StringFixedBank(3), p.Cfg.Coin)
		} else if profitPoint.LessThanOrEqual(profit) {
			sellQty = coinQty.Mul(decimal.NewFromFloat(0.6))
			msg = fmt.Sprintf("达到止盈点，尝试卖出%s个%s", sellQty.StringFixedBank(3), p.Cfg.Coin)
		} else if profit.LessThanOrEqual(lossPoint.Mul(decimal.NewFromFloat(1.5))) {
			msg = fmt.Sprintf("达到1.5倍止损点，尝试卖出%s个%s", sellQty.StringFixedBank(3), p.Cfg.Coin)
			sellQty = coinQty
		} else if profit.LessThanOrEqual(lossPoint) {
			sellQty = coinQty.Mul(decimal.NewFromFloat(0.8))
			msg = fmt.Sprintf("达到止损点，尝试卖出%s个%s", sellQty.StringFixedBank(3), p.Cfg.Coin)
		}
	}
	if decimal.NewFromFloat(0.1).LessThan(sellQty) && atomic.CompareAndSwapInt32(&p.sellLock, 0, 1) {
		defer func() {
			go time.AfterFunc(time.Second*time.Duration(p.Cfg.SellInterval), func() {
				atomic.StoreInt32(&p.sellLock, 0)
			})
		}()
		p.Log.Infof(msg)
		if err := p.Sell(sellQty); err != nil {
			p.Log.Error("sell err", err)
			return
		}
		p.Pusher.Push(msg)
	}
}
