package executor

import (
	"context"
	"fmt"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	k8sLabelApp       = "flow-ci-app"
	k8sLabelName      = "flow-ci-app-name"
	k8sLabelValueStep = "step"

	k8sDefaultStartPodTimeout = 30 * time.Second
)

type (
	K8sExecutor struct {
		BaseExecutor

		K8sConfig *rest.Config
		client    *kubernetes.Clientset
		namespace string
		workDir   string
		envFile   string

		pod *v1.Pod
	}
)

func (k *K8sExecutor) Init() (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = err.(error)
		}
	}()

	k.result.StartAt = time.Now()

	k.initK8s()
	k.initConfig()

	return
}

func (k *K8sExecutor) Start() (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = k.handleErrors(err.(error))
		}
	}()

	pod := k.createPodConfig()

	// start pod
	pod, err := k.client.CoreV1().Pods(k.namespace).Create(k.context, pod, metav1.CreateOptions{})
	util.PanicIfErr(err)
	k.pod = pod

	k.waitForRunning(k8sDefaultStartPodTimeout)
	k.copyPlugins()

	return
}

func (k *K8sExecutor) StartTty(ttyId string, onStarted func(ttyId string)) (out error) {
	return
}

func (k *K8sExecutor) StopTty() {
}

//--------------------------------------------
// private methods
//--------------------------------------------

func (k *K8sExecutor) initK8s() {
	if k.K8sConfig == nil {
		config, err := rest.InClusterConfig()
		util.PanicIfErr(err)
		k.K8sConfig = config
	}

	client, err := kubernetes.NewForConfig(k.K8sConfig)
	util.PanicIfErr(err)

	k.client = client
	k.namespace = k.getNamespace()
}

func (k *K8sExecutor) initConfig() {
	// set job work dir in the container = /ws/{flow id}
	k.workDir = filepath.Join(dockerWorkspace, util.ParseString(k.inCmd.FlowId))

	k.vars[domain.VarAgentWorkspace] = dockerWorkspace
	k.vars[domain.VarAgentJobDir] = k.workDir
	k.vars[domain.VarAgentPluginDir] = dockerPluginDir
}

func (k *K8sExecutor) getNamespace() string {
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns
	}

	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}

	return "default"
}

func (k *K8sExecutor) createPodConfig() *v1.Pod {
	dockers := k.inCmd.Dockers
	containers := make([]v1.Container, len(dockers))
	var runtimeContainer *v1.Container

	// setup containers
	for i, option := range dockers {
		containers[i] = v1.Container{
			Name:            option.Name,
			Image:           option.Image,
			Env:             k.toEnvVar(option.Environment),
			Ports:           k.toPort(option.Ports),
			Command:         option.Command,
			ImagePullPolicy: "Always",
		}

		if option.IsRuntime {
			runtimeContainer = &containers[i]
		}
	}

	if runtimeContainer == nil {
		if len(dockers) > 1 {
			panic(fmt.Errorf("no runtime docker option available"))
		}
		runtimeContainer = &containers[0]
	}

	// setup runtime container
	runtimeContainer.WorkingDir = k.workDir
	runtimeContainer.Stdin = false
	runtimeContainer.TTY = false
	runtimeContainer.Env = append(runtimeContainer.Env, k.toEnvVar(k.vars.Resolve())...)
	if runtimeContainer.Command == nil {
		runtimeContainer.Command = []string{linuxBash}
	}

	// create pod config
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: runtimeContainer.Name, // pod name as runtime container name
			Labels: map[string]string{
				k8sLabelApp:  k8sLabelValueStep,
				k8sLabelName: runtimeContainer.Name,
			},
		},
		Spec: v1.PodSpec{
			Containers:    containers,
			RestartPolicy: "Never",
		},
	}
}

func (k *K8sExecutor) waitForRunning(timeout time.Duration) {
	podName := k.pod.Name

	watch, err := k.client.CoreV1().Pods(k.namespace).Watch(k.context, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", k8sLabelName, podName),
	})
	util.PanicIfErr(err)

	startTime := time.Now()
	for {
		select {
		case event := <-watch.ResultChan():
			pod := event.Object.(*v1.Pod)
			util.LogInfo("Pod %s: status = %s", podName, pod.Status.Phase)

			if pod.Status.Phase == v1.PodPending {
				if time.Now().Sub(startTime) > timeout {
					panic(fmt.Errorf("start pod %s timeout on pending", podName))
				}
				break
			}

			if pod.Status.Phase == v1.PodRunning {
				util.LogInfo("Pod %s is running", pod.Name)
				return
			}

			if pod.Status.Phase == v1.PodFailed {
				panic(fmt.Errorf("start pod %s failed", podName))
			}

		case <-time.After(timeout):
			panic(fmt.Errorf("start pod %s timeout", podName))
		}
	}
}

func (k *K8sExecutor) copyPlugins() {

}

func (k *K8sExecutor) handleErrors(err error) error {
	if err == context.DeadlineExceeded {
		util.LogDebug("Timeout..")
		//kill()
		k.toTimeOutStatus()
		k.context = context.Background() // reset context for further docker operation
		return nil
	}

	if err == context.Canceled {
		util.LogDebug("Cancel..")
		//kill()
		k.toKilledStatus()
		k.context = context.Background() // reset context for further docker operation
		return nil
	}

	_ = k.toErrorStatus(err)
	return err
}

func (k *K8sExecutor) toEnvVar(vars domain.Variables) []v1.EnvVar {
	if vars == nil {
		return []v1.EnvVar{}
	}

	k8sVars := make([]v1.EnvVar, len(vars))

	i := 0
	for k, v := range vars {
		k8sVars[i] = v1.EnvVar{
			Name:  k,
			Value: v,
		}
		i++
	}

	return k8sVars
}

// toPort will apply container port only if HOST:CONTAINER
//ports:
//- "3000"
//- "8000:8000"
func (k *K8sExecutor) toPort(ports []string) []v1.ContainerPort {
	if ports == nil {
		return []v1.ContainerPort{}
	}

	containerPorts := make([]v1.ContainerPort, len(ports))
	for i, port := range ports {
		index := strings.IndexByte(port, ':')
		if index != -1 {
			port = port[index+1:]
		}

		intPort, _ := strconv.Atoi(port)
		containerPorts[i] = v1.ContainerPort{
			ContainerPort: int32(intPort),
		}
	}

	return containerPorts
}
