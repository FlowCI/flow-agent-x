package executor

import (
	"bytes"
	"container/list"
	"context"
	"encoding/base64"
	"fmt"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/exec"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	k8sLabelApp               = "flow-ci-app"
	k8sLabelName              = "flow-ci-app-name"
	k8sLabelValueStep         = "step"
	k8sDefaultStartPodTimeout = 120 * time.Second
)

type (
	K8sExecutor struct {
		BaseExecutor

		inCluster    bool
		agentPodName string
		namespace    string

		config *rest.Config
		client *kubernetes.Clientset

		workDir string
		envFile string

		pod     *v1.Pod
		runtime *v1.Container
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

		k.cleanupPod()
		k.closeChannels()
	}()

	pod := k.createPodConfig()

	// start pod
	pod, err := k.client.CoreV1().Pods(k.namespace).Create(k.context, pod, metav1.CreateOptions{})
	util.PanicIfErr(err)

	// setup pod and runtime container
	for _, c := range pod.Spec.Containers {
		if c.Name == pod.Name {
			k.runtime = &c
			break
		}
	}
	k.pod = pod

	k.waitForRunning(k8sDefaultStartPodTimeout)
	k.copyPlugins()
	k.runShell()

	k.setProcessId()
	k.exportEnv()

	if k.IsInteracting() {
		util.LogDebug("Tty is running, wait..")
		<-k.ttyCtx.Done()
	}

	k.toFinishStatus(0)
	return
}

func (k *K8sExecutor) StartTty(ttyId string, onStarted func(ttyId string)) (out error) {
	if k.IsInteracting() {
		return fmt.Errorf("interaction is ongoning")
	}

	if !k.isPodRunning() {
		return fmt.Errorf("pod is not running")
	}

	k.ttyId = ttyId
	k.ttyCtx, k.ttyCancel = context.WithCancel(k.context)

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()

	defer func() {
		if err := recover(); err != nil {
			_, _ = outWriter.Write([]byte(err.(error).Error()))
		}

		_ = inReader.Close()
		_ = inWriter.Close()

		_ = outReader.Close()
		_ = outWriter.Close()

		k.ttyCancel()
		k.ttyCtx = nil
		k.ttyCancel = nil
	}()

	onStarted(ttyId)

	go func() {
		_, _ = inWriter.Write([]byte(writeTtyPid))
		k.writeTtyIn(inWriter)
	}()
	go k.writeTtyOut(outReader)

	k.execInRuntimeContainer([]string{linuxBash}, true, inReader, outWriter, outWriter)
	return
}

func (k *K8sExecutor) StopTty() {
	k.runSingleCmd(killTty + "\n")
}

//--------------------------------------------
// private methods
//--------------------------------------------

func (k *K8sExecutor) initK8s() {
	if k.inCluster {
		config, err := rest.InClusterConfig()
		util.PanicIfErr(err)
		k.config = config
	} else {
		// load config from ~/.kube/config
		homeKubeConfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
		config, err := clientcmd.BuildConfigFromFlags("", homeKubeConfig)
		util.PanicIfErr(err)
		k.config = config
	}

	client, err := kubernetes.NewForConfig(k.config)
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
	if k.namespace != "" {
		return k.namespace
	}

	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}

	return "default"
}

// create pod config, pod name = runtime container name
func (k *K8sExecutor) createPodConfig() *v1.Pod {
	dockers := k.inCmd.Dockers
	containers := list.New()

	var podVolumes []v1.Volume
	var runtimeContainer *v1.Container

	// setup containers
	for _, option := range dockers {
		c := &v1.Container{
			Name:            option.Name,
			Image:           option.Image,
			Env:             k.toEnvVar(option.Environment),
			Ports:           k.toPort(option.Ports),
			Command:         option.Command,
			ImagePullPolicy: "Always",
		}

		if option.IsRuntime {
			runtimeContainer = c
		}

		containers.PushBack(c)
	}

	if runtimeContainer == nil {
		if len(dockers) > 1 {
			panic(fmt.Errorf("no runtime docker option available"))
		}
		runtimeContainer = containers.Front().Value.(*v1.Container)
	}

	// setup runtime container
	runtimeContainer.WorkingDir = k.workDir
	runtimeContainer.Stdin = true
	runtimeContainer.TTY = true
	runtimeContainer.Env = append(runtimeContainer.Env, k.toEnvVar(k.vars.Resolve())...)
	if runtimeContainer.Command == nil {
		runtimeContainer.Command = []string{linuxBash}
	}

	// setup container from volume
	for _, v := range k.volumes {
		if !v.HasImage() {
			continue
		}

		volumeName := fmt.Sprintf("%s-share", v.Name)

		// setup runtime container volume
		runtimeContainer.VolumeMounts = append(runtimeContainer.VolumeMounts, v1.VolumeMount{
			Name:      volumeName,
			MountPath: v.Dest,
		})

		// setup pod volume
		podVolumes = append(podVolumes, v1.Volume{
			Name: volumeName,
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		})

		containers.PushBack(&v1.Container{
			Name:    v.Name,
			Image:   v.Image,
			Command: []string{v.InitScriptInImage()},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      volumeName,
					MountPath: v.DefaultTargetInImage(),
				},
			},
			ImagePullPolicy: "Always",
		})
	}

	// setup docker dind
	runtimeContainer.Env = append(runtimeContainer.Env, v1.EnvVar{
		Name:  "DOCKER_HOST",
		Value: "tcp://localhost:2375",
	})
	containers.PushBack(k8sDinDContainer())

	// create pod config
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: runtimeContainer.Name,
			Labels: map[string]string{
				k8sLabelApp:  k8sLabelValueStep,
				k8sLabelName: runtimeContainer.Name,
			},
		},
		Spec: v1.PodSpec{
			Containers:    toContainerArray(containers),
			Volumes:       podVolumes,
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
			k.pod = pod // update pod instance

			k.writeSingleLog(fmt.Sprintf("Pod %s: status = %s", podName, pod.Status.Phase))

			if pod.Status.Phase == v1.PodPending {
				if time.Now().Sub(startTime) > timeout {
					panic(fmt.Errorf("start pod %s timeout on pending", podName))
				}
				break
			}

			if pod.Status.Phase == v1.PodRunning {
				k.writeSingleLog(fmt.Sprintf("Pod %s is running", pod.Name))
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
	if util.IsEmptyString(k.pluginDir) {
		return
	}

	reader, err := tarArchiveFromPath(k.pluginDir)
	util.PanicIfErr(err)

	cmd := []string{"tar", "-xf", "-", "-C", dockerWorkspace}
	k.execInRuntimeContainer(cmd, false, reader, os.Stdout, os.Stderr)
}

func (k *K8sExecutor) runShell() {
	k.stdOutWg.Add(1)

	input := bytes.NewBuffer(make([]byte, 2048))
	reader, writer := io.Pipe()

	defer func() {
		_ = reader.Close()
		_ = writer.Close()
	}()

	setupContainerIpAndBin := func(in chan string) {
		for i, _ := range k.pod.Spec.Containers {
			in <- fmt.Sprintf(domain.VarExportContainerIdPattern, i, "localhost")
			in <- fmt.Sprintf(domain.VarExportContainerIpPattern, i, "127.0.0.1")
		}

		in <- fmt.Sprintf("mkdir -p %s", dockerBin)
		in <- fmt.Sprintf("export PATH=%s:$PATH", dockerBin)

		for _, f := range binFiles {
			path := dockerBin + "/" + f.name
			b64 := base64.StdEncoding.EncodeToString(f.content)
			in <- fmt.Sprintf("echo -e \"%s\" | base64 -d > %s", b64, path)
			in <- fmt.Sprintf("chmod %s %s", f.permissionStr, path)
		}
	}

	writeEnvAfter := func(in chan string) {
		in <- "env > " + dockerEnvFile
	}

	_, _ = input.Write([]byte(writeShellPid))

	k.writeLog(reader, true, true)
	k.writeCmd(input, setupContainerIpAndBin, writeEnvAfter, k8sSetDockerNetwork)

	k.toStartStatus(0)
	k.execInRuntimeContainer([]string{linuxBash}, false, input, writer, writer)
}

func (k *K8sExecutor) setProcessId() {
	input := bytes.NewBufferString(fmt.Sprintf("cat %s", dockerShellPidPath))
	buffer := bytes.NewBuffer(make([]byte, 10))

	k.execInRuntimeContainer([]string{linuxBash}, false, input, buffer, nil)

	data, err := ioutil.ReadAll(buffer)
	if err == nil {
		num, _ := strconv.Atoi(string(trimByte(data)))
		k.result.ProcessId = num
	}
}

// read env file from container and set output
func (k *K8sExecutor) exportEnv() {
	input := bytes.NewBufferString(fmt.Sprintf("cat %s", dockerEnvFile))
	buffer := bytes.NewBuffer(make([]byte, 4096))

	k.execInRuntimeContainer([]string{linuxBash}, false, input, buffer, nil)
	k.result.Output = readEnvFromReader(buffer, k.inCmd.EnvFilters)
}

func (k *K8sExecutor) cleanupPod() {
	if k.client == nil || k.pod == nil {
		return
	}

	background := context.Background() // set new context since context cancelled error may occurred
	err := k.client.CoreV1().Pods(k.namespace).Delete(background, k.pod.Name, metav1.DeleteOptions{})

	if err != nil {
		util.LogWarn(err.Error())
	}
}

func (k *K8sExecutor) runSingleCmd(cmd string) {
	defer func() {
		if err := recover(); err != nil {
			util.LogWarn("Error for cmd %s: %s", cmd, err.(error).Error())
		}
	}()

	input := bytes.NewBuffer([]byte(cmd))
	k.execInRuntimeContainer([]string{linuxBash}, false, input, nil, nil)
}

// exec command in the runtime container
// should get error if executed with non-zero exit code
func (k *K8sExecutor) execInRuntimeContainer(cmd []string, tty bool, stdin io.Reader, stdout io.Writer, stderr io.Writer) {
	options := &v1.PodExecOptions{
		Container: k.runtime.Name,
		Command:   cmd,
		Stdin:     stdin != nil,
		Stdout:    stdout != nil,
		Stderr:    stderr != nil,
		TTY:       tty,
	}

	req := k.client.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(k.pod.Name).
		Namespace(k.namespace).
		SubResource("exec").
		VersionedParams(options, scheme.ParameterCodec)

	k8sExec, err := remotecommand.NewSPDYExecutor(k.config, "POST", req.URL())
	util.PanicIfErr(err)

	err = k8sExec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    tty,
	})
	util.PanicIfErr(err)
}

func (k *K8sExecutor) isPodRunning() bool {
	return k.pod != nil && k.pod.Status.Phase == v1.PodRunning
}

func (k *K8sExecutor) handleErrors(err error) error {
	kill := func() {
		k.runSingleCmd(killShell + "\n")
		k.runSingleCmd(killTty + "\n")
	}

	// err from exec when got non-zero exit code
	if exitError, ok := err.(exec.CodeExitError); ok {
		k.toFinishStatus(exitError.Code)
		return nil
	}

	if err == context.DeadlineExceeded {
		util.LogDebug("Timeout..")
		kill()
		k.toTimeOutStatus()
		k.context = context.Background() // reset context for further docker operation
		return nil
	}

	if err == context.Canceled {
		util.LogDebug("Cancel..")
		kill()
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
