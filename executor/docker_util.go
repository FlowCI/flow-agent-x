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
	dockerHeaderSize       = 8
	dockerHeaderPrefixSize = 4 // [STREAM_TYPE, 0, 0 ,0, ....]
)

var (
	dockerStdInHeaderPrefix  = []byte{1, 0, 0, 0}
	dockerStdErrHeaderPrefix = []byte{2, 0, 0, 0}
)

// is repo link belong to the format <hub-user>/<repo-name>
func isDockerHubImage(image string) bool {
	items := strings.Split(image, "/")
	if len(items) <= 2 {
		return true
	}
	return false
}

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

func untarFromReader(tarReader io.Reader, dest string) error {
	reader := tar.NewReader(tarReader)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		fileInfo := header.FileInfo()
		target := dest + util.UnixPathSeparator + header.Name

		if fileInfo.IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}

		f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
		if err != nil {
			return err
		}

		if _, err := io.Copy(f, reader); err != nil {
			return err
		}

		f.Close()
	}

	return nil
}

// tar dir, ex: abc/.. output is archived content .. in dir
func tarArchiveFromPath(path string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	dir := filepath.Dir(path)

	ok := filepath.Walk(path, func(file string, fi os.FileInfo, err error) (out error) {
		defer util.RecoverPanic(func(e error) {
			out = e
		})

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
