package main

import (
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
	// log.SetReportCaller(true)
}

func main() {
	log.Info("Starting flow.ci agent....")
}
