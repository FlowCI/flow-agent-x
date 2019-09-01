package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
			Value:  "http://127.0.0.1:8080",
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
			Usage:  "Port for agent",
			EnvVar: "FLOWCI_AGENT_PORT",
		},

		cli.StringFlag{
			Name:   "workspace, w",
			Value:  filepath.Join("${HOME}", ".flow.ci.agent"),
			Usage:  "Agent working directory",
			EnvVar: "FLOWCI_AGENT_WORKSPACE",
		},

		cli.StringFlag{
			Name:   "plugindir, pd",
			Value:  filepath.Join("${HOME}", ".flow.ci.agent", "plugins"),
			Usage:  "Directory for plugin",
			EnvVar: "FLOWCI_AGENT_PLUGIN_DIR",
		},

		cli.StringFlag{
			Name:   "logdir, ld",
			Value:  filepath.Join("${HOME}", ".flow.ci.agent", "logs"),
			Usage:  "Directory for plugin",
			EnvVar: "FLOWCI_AGENT_LOG_DIR",
		},
	}

	err := app.Run(os.Args)
	util.LogIfError(err)
}

func start(c *cli.Context) error {
	util.LogInfo("Staring flow.ci agent...")

	// try to load config from server
	config := config.GetInstance()
	config.Server = c.String("url")
	config.Token = c.String("token")
	config.Port = getPort(c.String("port"))
	config.Workspace = util.ParseString(c.String("workspace"))
	config.PluginDir = util.ParseString(c.String("plugindir"))
	config.LoggingDir = util.ParseString(c.String("logdir"))
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

func getPort(strPort string) int {
	if util.IsEmptyString(strPort) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		util.FailOnError(err, "Cannot start listen localhost")
		defer func() {
			_ = listener.Close()
		}()

		addressAndPort := listener.Addr().String()

		strPort = addressAndPort[strings.Index(addressAndPort, ":")+1:]
		util.LogDebug("Port = " + strPort)
	}

	i, err := strconv.Atoi(strPort)
	util.FailOnError(err, "Invalid port format")
	return i
}
