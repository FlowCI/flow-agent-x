package util

import (
	"fmt"
	"log"

	logger "github.com/sirupsen/logrus"
)

// FailOnError exit program with err
func FailOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func LogIfError(err error) {
	if err != nil {
		logger.Error(err)
	}
}

func LogInfo(format string, a ...interface{}) {
	str := fmt.Sprintf(format, a...)
	logger.Info(str)
}

func LogDebug(format string, a ...interface{}) {
	str := fmt.Sprintf(format, a...)
	logger.Debug(str)
}
