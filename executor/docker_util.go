package executor

import (
	"archive/tar"
	"bufio"
	"bytes"
	"container/list"
	"github/flowci/flow-agent-x/util"
	"io"
	v1 "k8s.io/api/core/v1"
	"os"
	"path/filepath"
	"strings"
)

const (
	dockerHeaderSize       = 8
	dockerHeaderPrefixSize = 4 // [STREAM_TYPE, 0, 0 ,0, ....]
)

var (
	dockerStdInHeaderPrefix  = []byte{1, 0, 0, 0}
	dockerStdErrHeaderPrefix = []byte{2, 0, 0, 0}
)

func removeDockerHeader(in []byte) []byte {
	if len(in) < dockerHeaderSize {
		return in
	}

	if bytes.Compare(in[:dockerHeaderPrefixSize], dockerStdInHeaderPrefix) == 0 {
		return in[dockerHeaderSize:]
	}

	if bytes.Compare(in[:dockerHeaderPrefixSize], dockerStdErrHeaderPrefix) == 0 {
		return in[dockerHeaderSize:]
	}

	return in
}

func toContainerArray(v1List *list.List) []v1.Container {
	array := make([]v1.Container, v1List.Len())
	i := 0
	for e := v1List.Front(); e != nil; e = e.Next() {
		c := e.Value.(*v1.Container)
		array[i] = *c
		i++
	}
	return array
}

func k8sDinDContainer() *v1.Container {
	return &v1.Container{
		Name:  "dind-daemon",
		Image: "docker:dind",
		SecurityContext: &v1.SecurityContext{
			Privileged: util.PointerBoolean(true),
		},
		Env: []v1.EnvVar{
			{
				Name:  "DOCKER_TLS_CERTDIR", // disable tls
				Value: "",
			},
		},
	}
}

func k8sSetDockerNetwork(script string) string {
	index := strings.Index(script, "docker run")
	if index == -1 {
		return script
	}

	if !strings.Contains(script[index:], "--network=") {
		return strings.Replace(script, "docker run", "docker run --network=host", 1)
	}

	start := strings.Index(script, "--network=")
	end := util.IndexOfFirstSpace(script[start:]) - 1

	if end < 0 {
		return script
	}

	return strings.Replace(script, script[start:end], "--network=host", 1)
}

// tar dir, ex: abc/.. output is archived content .. in dir
func tarArchiveFromPath(path string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	dir := filepath.Dir(path)

	ok := filepath.Walk(path, func(file string, fi os.FileInfo, err error) (out error) {
		defer func() {
			if err := recover(); err != nil {
				out = err.(error)
			}
		}()
		util.PanicIfErr(err)

		header, err := tar.FileInfoHeader(fi, fi.Name())
		util.PanicIfErr(err)

		relativeDir := strings.Replace(file, dir, "", -1)
		header.Name = strings.TrimPrefix(relativeDir, string(filepath.Separator))

		// convert path to linux path
		if util.IsWindows() {
			header.Name = strings.ReplaceAll(header.Name, util.WinPathSeparator, util.UnixPathSeparator)
		}

		err = tw.WriteHeader(header)
		util.PanicIfErr(err)

		f, err := os.Open(file)
		util.PanicIfErr(err)

		if fi.IsDir() {
			return
		}

		_, err = io.Copy(tw, f)
		util.PanicIfErr(err)

		err = f.Close()
		util.PanicIfErr(err)

		return
	})

	if ok != nil {
		return nil, ok
	}

	ok = tw.Close()
	if ok != nil {
		return nil, ok
	}

	return bufio.NewReader(&buf), nil
}
