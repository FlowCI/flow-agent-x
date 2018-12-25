package main

import (
	"fmt"

	"github.com/flowci/flow-agent-x/config"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
	// log.SetReportCaller(true)
}

func hello() {
	fmt.Println("Hello world goroutine")
}

func main() {
	log.Info("Starting flow.ci agent....")

	config := config.GetInstance()
	config.Init()
	defer config.Close()
}
