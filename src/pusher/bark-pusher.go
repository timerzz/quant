package pusher

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/timerzz/go-quant/src/cfg"
	"io/ioutil"
	"net/http"
)

type Bark struct {
	cfg cfg.BarkCfg
}

func NewBarkPush(cfg cfg.BarkCfg) *Bark {
	return &Bark{cfg: cfg}
}

func (b *Bark) Push(msg string) {
	res, err := http.Get(fmt.Sprintf("http://%s/%s/%s", b.cfg.Url, b.cfg.Token, msg))
	if err != nil {
		logrus.Errorf("push %s err: %v", msg, err)
		return
	}
	_, err = ioutil.ReadAll(res.Body)
	if err != nil {
		logrus.Errorf("push %s err: %v", msg, err)
		return
	}
}
