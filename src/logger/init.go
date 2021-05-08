package logger

import (
	"github.com/timerzz/go-quant/src/cfg"
	"io"
	"os"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
)

var (
	RuningLogWriter io.Writer
)

func Init(cfg cfg.LogCfg) {
	RuningLogWriter, _ = rotatelogs.New(
		cfg.RunLog+".%Y%m%d%H",
		rotatelogs.WithLinkName(cfg.RunLog),
		rotatelogs.WithMaxAge(time.Duration(24*7)*time.Hour),
		rotatelogs.WithRotationTime(time.Duration(24)*time.Hour),
	)
	fileAndStdoutWriter := io.MultiWriter(RuningLogWriter, os.Stdout)
	logrus.SetOutput(fileAndStdoutWriter)
	logrus.Infof("log inited:%s", cfg.RunLog)
}
