package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/urfave/cli"
	"github/flowci/flow-agent-x/config"
	"github/flowci/flow-agent-x/controller"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
)

const version = "1.21.50"

func init() {
	util.LogInit()
	util.EnableDebugLog()
}

func main() {
	app := cli.NewApp()
	app.Name = "Agent of flow.ci"
	app.Usage = ""
	app.Action = start
	app.Author = "yang.guo"
	app.Version = version

	cm := config.GetInstance()

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "debug, d",
			Usage:       "Enable debug model",
			EnvVar:      domain.VarAgentDebug,
			Destination: &cm.Debug,
		},

		cli.StringFlag{
			Name:        "url, u",
			Value:       "http://127.0.0.1:8080",
			Usage:       "flow.ci server url",
			EnvVar:      domain.VarServerUrl,
			Destination: &cm.Server,
		},

		cli.StringFlag{
			Name:        "token, t",
			Usage:       "Token for agent",
			EnvVar:      domain.VarAgentToken,
			Destination: &cm.Token,
		},

		cli.IntFlag{
			Name:        "port, p",
			Value:       -1,
			Usage:       "Port for agent",
			EnvVar:      domain.VarAgentPort,
			Destination: &cm.Port,
		},

		cli.StringFlag{
			Name:        "profile",
			Usage:       "Enable or disable agent profiling",
			EnvVar:      domain.VarAgentEnableProfile,
			Value:       "true",
			Destination: &cm.ProfileEnabledStr,
		},

		cli.BoolFlag{
			Name:        "k8sEnabled",
			Usage:       "Indicate is run from k8s",
			EnvVar:      domain.VarK8sEnabled,
			Destination: &cm.K8sEnabled,
		},

		cli.BoolFlag{
			Name:        "k8sInCluster",
			Usage:       "Indicate is k8s run in cluster",
			EnvVar:      domain.VarK8sInCluster,
			Destination: &cm.K8sCluster,
		},

		cli.StringFlag{
			Name:        "workspace, w",
			Value:       filepath.Join(util.HomeDir, ".flow.ci.agent"),
			Usage:       "Agent working directory",
			EnvVar:      domain.VarAgentWorkspace,
			Destination: &cm.Workspace,
		},

		cli.StringFlag{
			Name: "volumes, m",
			Usage: "List of volume that will mount to docker from step \n" +
				"format: name=xxx,dest=xxx,script=xxx;name=xxx,dest=xxx,script=xxx;...",
			EnvVar:      domain.VarAgentVolumes,
			Destination: &cm.VolumesStr,
		},
	}

	err := app.Run(os.Args)
	util.LogIfError(err)
}

func start(c *cli.Context) error {
	util.LogInfo("Staring flow.ci agent (v%s)...", version)
	defer func() {
		if err := recover(); err != nil {
			util.LogIfError(err.(error))
		}
		util.LogInfo("Agent stopped")
	}()

	cm := config.GetInstance()
	cm.Init()

	defer cm.Close()

	// connect to ci server
	startGin(cm)

	return nil
}

func startGin(cm *config.Manager) {
	router := gin.Default()
	controller.NewCmdController(router)
	controller.NewHealthController(router)

	if cm.Debug {
		pprof.Register(router)
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cm.Port),
		Handler: router,
	}

	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			util.FailOnError(err, "Unable to listen")
		}
	}()

	// wait
	<-cm.AppCtx.Done()

	if err := server.Shutdown(cm.AppCtx); err != nil {
		util.FailOnError(err, "Unable to stop the agent")
	}
}
