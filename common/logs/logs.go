package logs

import (
	"common/config"
	"github.com/charmbracelet/log"
	"os"
	"time"
)

var logger *log.Logger

func InitLog(appName string) {
	logger = log.New(os.Stderr)
	if config.Conf.Log.Level == "DEBUG" {
		logger.SetLevel(log.DebugLevel)
	} else {
		logger.SetLevel(log.InfoLevel)
	}
	logger.SetPrefix(appName)
	logger.SetReportTimestamp(true)
	logger.SetTimeFormat(time.DateTime)
}

func Fatal(format string, values ...any) {
	if len(values) == 0 {
		logger.Fatal(format)
	} else {
		logger.Fatalf(format, values...)
	}
}
func Info(format string, values ...any) {
	if len(values) == 0 {
		logger.Info(format)
	} else {
		logger.Infof(format, values...)
	}
}

func Warn(format string, values ...any) {
	if len(values) == 0 {
		logger.Warn(format)
	} else {
		logger.Warnf(format, values...)
	}
}

func Debug(format string, values ...any) {
	if len(values) == 0 {
		logger.Debug(format)
	} else {
		logger.Debugf(format, values...)
	}
}
func Error(format string, values ...any) {
	if len(values) == 0 {
		logger.Error(format)
	} else {
		logger.Errorf(format, values...)
	}
}
