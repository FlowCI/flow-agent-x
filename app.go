package main

import (
	"github.com/flowci/flow-agent-x/config"
	"github.com/flowci/flow-agent-x/controller"
	"github.com/flowci/flow-agent-x/util"
	"github.com/gin-gonic/gin"
)

func init() {
	util.LogInit()
	util.EnableDebugLog()
}

func main() {
	util.LogInfo("Staring agent of flow.ci...")

	// try to load config from server
	config := config.GetInstance()
	config.Init()

	defer config.Close()

	// start agent
	router := gin.Default()
	controller.NewCmdController(router)
	router.Run(":8000")
}
