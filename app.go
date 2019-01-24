package main

import (
	"context"
	"net/http"
	"time"

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
	controller.NewHealthController(router)

	server := &http.Server{
		Addr:    ":8000",
		Handler: router,
	}

	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			util.FailOnError(err, "unable to listen")
		}
	}()

	<-config.Quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		util.FailOnError(err, "unable to shutdown server")
	}

	util.LogInfo("agent: shutdown ...")
}
