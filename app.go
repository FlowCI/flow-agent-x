package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"github/flowci/flow-agent-x/config"
	"github/flowci/flow-agent-x/controller"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/executor"
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

const version = "0.20.45"

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
			Value:  "",
			Usage:  "Port for agent",
			EnvVar: domain.VarAgentPort,
		},

		cli.BoolFlag{
			Name:   "k8sEnabled",
			Usage:  "Indicate is run from k8s",
			EnvVar: domain.VarK8sEnabled,
		},

		cli.BoolFlag{
			Name:   "k8sInCluster",
			Usage:  "Indicate is k8s run in cluster",
			EnvVar: domain.VarK8sInCluster,
		},

		cli.StringFlag{
			Name:   "workspace, w",
			Value:  filepath.Join(util.HomeDir, ".flow.ci.agent"),
			Usage:  "Agent working directory",
			EnvVar: domain.VarAgentWorkspace,
		},

		cli.StringFlag{
			Name: "volumes, m",
			Usage: "List of volume that will mount to docker from step \n" +
				"format: name=xxx,dest=xxx,script=xxx;name=xxx,dest=xxx,script=xxx;...",
			EnvVar: domain.VarAgentVolumes,
		},

		cli.StringFlag{
			Name:  "script",
			Value: "",
			Usage: "Execute shell script locally, ex: --script \"echo hello world\"",
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

	config := config.GetInstance()
	defer config.Close()

	config.Server = c.String("url")
	config.Token = c.String("token")
	config.Port = getPort(c.String("port"))
	config.Workspace = util.ParseString(c.String("workspace"))
	config.PluginDir = filepath.Join(config.Workspace, ".plugins")
	config.LoggingDir = filepath.Join(config.Workspace, ".logs")
	config.VolumesStr = c.String("volumes")

	config.K8sEnabled = c.Bool("k8sEnabled")
	config.K8sCluster = c.Bool("k8sInCluster")

	config.K8sNodeName = os.Getenv(domain.VarK8sNodeName)
	config.K8sPodName = os.Getenv(domain.VarK8sPodName)
	config.K8sPodIp = os.Getenv(domain.VarK8sPodIp)
	config.K8sNamespace = os.Getenv(domain.VarK8sNamespace)

	printInfo()

	// exec given cmd
	script := c.String("script")
	if len(script) > 0 {
		execCmd(script)
		return nil
	}

	// connect to ci server
	config.Init()
	startGin(config)

	return nil
}

func printInfo() {
	appConfig := config.GetInstance()

	util.LogInfo("--- [Server URL]: %s", appConfig.Server)
	util.LogInfo("--- [Token]: %s", appConfig.Token)
	util.LogInfo("--- [Port]: %d", appConfig.Port)
	util.LogInfo("--- [Workspace]: %s", appConfig.Workspace)
	util.LogInfo("--- [Plugin Dir]: %s", appConfig.PluginDir)
	util.LogInfo("--- [Log Dir]: %s", appConfig.LoggingDir)
	util.LogInfo("--- [Volume Str]: %s", appConfig.VolumesStr)

	if appConfig.K8sEnabled {
		util.LogInfo("--- [K8s InCluster]: %d", appConfig.K8sCluster)
		util.LogInfo("--- [K8s Node]: %s", appConfig.K8sNodeName)
		util.LogInfo("--- [K8s Namespace]: %s", appConfig.K8sNamespace)
		util.LogInfo("--- [K8s Pod]: %s", appConfig.K8sPodName)
		util.LogInfo("--- [K8s Pod IP]: %s", appConfig.K8sPodIp)
	}
}

func execCmd(script string) {
	cmd := &domain.ShellIn{
		ID:      "local",
		Bash:    []string{script},
		Inputs:  domain.Variables{},
		Timeout: 1800,
	}

	printer := func(channel <-chan string) {
		for b64 := range channel {
			bytes, _ := base64.StdEncoding.DecodeString(b64)
			util.LogInfo("[LOG]: %s", string(bytes))
		}
	}

	bashExecutor := executor.NewExecutor(executor.Options{
		Parent: context.Background(),
		Cmd:    cmd,
	})

	go printer(bashExecutor.Stdout())
	_ = bashExecutor.Start()
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
	<-config.AppCtx.Done()

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
