package executor

import (
	"archive/tar"
	"bufio"
	"bytes"
	"github/flowci/flow-agent-x/util"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	dockerHeaderSize = 8
	dockerHeaderPrefixSize = 4 // [STREAM_TYPE, 0, 0 ,0, ....]
)

var (
	dockerStdInHeaderPrefix  = []byte{1, 0, 0, 0}
	dockerStdErrHeaderPrefix = []byte{2, 0, 0, 0}
)

func removeDockerHeader (in []byte) []byte {
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

		header.Name = strings.TrimPrefix(strings.Replace(file, dir, "", -1), string(filepath.Separator))
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
