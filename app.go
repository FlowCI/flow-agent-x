package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"flow-agent-x/config"
	"flow-agent-x/controller"
	"flow-agent-x/util"

	"github.com/gin-gonic/gin"
	"github.com/urfave/cli"
)

func init() {
	util.LogInit()
	util.EnableDebugLog()
}

func main() {
	app := cli.NewApp()
	app.Name = "Agent of flow.ci"
	app.Action = start
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "url, u",
			Value:  "127.0.0.1:8080",
			Usage:  "flow.ci server url",
			EnvVar: "FLOWCI_SERVER_URL",
		},

		cli.StringFlag{
			Name:   "token, t",
			Usage:  "Token for agent",
			EnvVar: "FLOWCI_AGENT_TOKEN",
		},

		cli.StringFlag{
			Name:   "port, p",
			Value:  "8088",
			Usage:  "Port for agent",
			EnvVar: "FLOWCI_AGENT_PORT",
		},
	}

	err := app.Run(os.Args)
	util.LogIfError(err)
}

func start(c *cli.Context) error {
	util.LogInfo("Staring flow.ci Agent...")

	// try to load config from server
	config := config.GetInstance()
	config.Server = c.String("url")
	config.Token = c.String("token")
	config.Port = c.Int("port")
	config.Init()

	defer config.Close()

	// start agent
	router := gin.Default()
	controller.NewCmdController(router)
	controller.NewHealthController(router)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: router,
	}

	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			util.FailOnError(err, "Unable to listen")
		}
	}()

	<-config.Quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		util.FailOnError(err, "Unable to stop the agent")
		return err
	}

	util.LogInfo("Agent stopped")
	return nil
}
