package executor

import (
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"time"
)

type (
	DockerK8sExecutor struct {
		BaseExecutor
		configs []*domain.DockerConfig
		client  *kubernetes.Clientset
		workDir string
		envFile string
	}
)

func (d *DockerK8sExecutor) Init() (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = err.(error)
		}
	}()

	d.result.StartAt = time.Now()

	config, err := rest.InClusterConfig()
	util.PanicIfErr(err)

	d.client, err = kubernetes.NewForConfig(config)
	util.PanicIfErr(err)

	//

	return
}
