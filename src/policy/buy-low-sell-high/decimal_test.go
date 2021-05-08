package buyLowSellHigh

import (
	"github.com/shopspring/decimal"
	"testing"
)

func TestToStringFix(t *testing.T) {
	d := decimal.NewFromFloat(0.237777777777777)
	t.Log(d.StringFixed(6))
}
