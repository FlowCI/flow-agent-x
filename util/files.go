package util

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func IsFileExists(path string) bool {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return true
	}
	return false
}

func Zip(src, dest, separator string) (out error) {
	defer func() {
		if r := recover(); r != nil {
			out = r.(error)
		}
	}()

	outFile, err := os.Create(dest)
	PanicIfErr(err)

	defer outFile.Close()

	w := zip.NewWriter(outFile)
	addFiles(w, src, "", separator)

	err = w.Close()
	PanicIfErr(err)
	return
}

func Unzip(src string, dest string) (out error) {
	defer func() {
		if r := recover(); r != nil {
			out = r.(error)
		}
	}()

	r, err := zip.OpenReader(src)
	PanicIfErr(err)
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			PanicIfErr(fmt.Errorf("%s: illegal file path", fpath))
		}

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm)
		PanicIfErr(err)

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		PanicIfErr(err)

		rc, err := f.Open()
		PanicIfErr(err)

		_, err = io.Copy(outFile, rc)
		PanicIfErr(err)

		outFile.Close()
		rc.Close()
	}

	return
}

func addFiles(w *zip.Writer, basePath, baseInZip, separator string) {
	files, err := ioutil.ReadDir(basePath)
	PanicIfErr(err)

	for _, file := range files {
		srcFullPath := basePath + separator + file.Name()

		if !file.IsDir() {
			dat, err := ioutil.ReadFile(srcFullPath)
			if err != nil {
				LogWarn(err.Error())
				continue
			}

			f, err := w.Create(baseInZip + file.Name())
			PanicIfErr(err)

			_, err = f.Write(dat)
			PanicIfErr(err)
			continue
		}

		// recurse on dir
		inZip := baseInZip + file.Name() + separator
		addFiles(w, srcFullPath, inZip, separator)
	}
}
