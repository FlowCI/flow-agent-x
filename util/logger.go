package util

import (
	"fmt"

	logger "github.com/sirupsen/logrus"
)

func LogInit() {
	logger.SetFormatter(&logger.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
}

func EnableDebugLog() {
	logger.SetLevel(logger.DebugLevel)
}

func LogIfError(err error) bool {
	if HasError(err) {
		logger.Error(err)
		return true
	}

	return false
}

func LogInfo(format string, a ...interface{}) {
	str := fmt.Sprintf(format, a...)
	logger.Info(str)
}

func LogDebug(format string, a ...interface{}) {
	str := fmt.Sprintf(format, a...)
	logger.Debug(str)
}

func LogWarn(format string, a ...interface{}) {
	str := fmt.Sprintf(format, a...)
	logger.Warn(str)
}
