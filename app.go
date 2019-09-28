package main

import (
	"fmt"
	"github/flowci/flow-agent-x/config"
	"github/flowci/flow-agent-x/controller"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/urfave/cli"
)

const version = "0.19.35"

func init() {
	util.LogInit()
	util.EnableDebugLog()
}

func main() {
	app := cli.NewApp()
	app.Name = "Agent of flow.ci"
	app.Action = start
	app.Version = version
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "url, u",
			Value:  "http://127.0.0.1:8080",
			Usage:  "flow.ci server url",
			EnvVar: domain.VarServerUrl,
		},

		cli.StringFlag{
			Name:   "token, t",
			Usage:  "Token for agent",
			EnvVar: domain.VarAgentToken,
		},

		cli.StringFlag{
			Name:   "port, p",
			Usage:  "Port for agent",
			EnvVar: domain.VarAgentPort,
		},

		cli.StringFlag{
			Name:   "workspace, w",
			Value:  filepath.Join("${HOME}", ".flow.ci.agent"),
			Usage:  "Agent working directory",
			EnvVar: domain.VarAgentWorkspace,
		},

		cli.StringFlag{
			Name:   "plugindir, pd",
			Value:  filepath.Join("${HOME}", ".flow.ci.agent", "plugins"),
			Usage:  "Directory for plugin",
			EnvVar: domain.VarAgentPluginDir,
		},

		cli.StringFlag{
			Name:   "logdir, ld",
			Value:  filepath.Join("${HOME}", ".flow.ci.agent", "logs"),
			Usage:  "Directory for logging",
			EnvVar: domain.VarAgentLogDir,
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

	startGin(config)
	util.LogInfo("Agent stopped")
	return nil
}

func startGin(config *config.Manager) {
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

	// wait
	<- config.AppCtx.Done()

	if err := server.Shutdown(config.AppCtx); err != nil {
		util.FailOnError(err, "Unable to stop the agent")
	}
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
