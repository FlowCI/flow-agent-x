package util

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

func IsFileExists(path string) bool {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return true
	}
	return false
}

func IsFileExistsAndReturnFileInfo(path string) (os.FileInfo, bool) {
	stat, err := os.Stat(path)
	if !os.IsNotExist(err) {
		return stat, true
	}
	return stat, false
}

// CopyFile The file will be created if it does not already exist. If the
// destination file exists, the contents will be replaced
func CopyFile(src, dst string) (errOut error) {
	defer RecoverPanic(func(e error) {
		errOut = e
	})

	in, err := os.Open(src)
	PanicIfErr(err)

	defer in.Close()

	out, err := os.Create(dst)
	PanicIfErr(err)

	defer func() {
		if e := out.Close(); e != nil {
			errOut = e
		}
	}()

	_, err = io.Copy(out, in)
	PanicIfErr(err)

	err = out.Sync()
	PanicIfErr(err)

	si, err := os.Stat(src)
	PanicIfErr(err)

	err = os.Chmod(dst, si.Mode())
	PanicIfErr(err)

	return
}

func CopyDir(src string, dst string) (err error) {
	defer RecoverPanic(nil)

	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	si, err := os.Stat(src)
	PanicIfErr(err)

	if !si.IsDir() {
		panic(fmt.Errorf("source is not a directory"))
	}

	_, err = os.Stat(dst)
	if err != nil && !os.IsNotExist(err) {
		return
	}

	if err == nil {
		panic(fmt.Errorf("destination already exists"))
	}

	err = os.MkdirAll(dst, si.Mode())
	PanicIfErr(err)

	entries, err := ioutil.ReadDir(src)
	PanicIfErr(err)

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = CopyDir(srcPath, dstPath)
			PanicIfErr(err)
			continue
		}

		// skip symlinks.
		if entry.Mode()&os.ModeSymlink != 0 {
			continue
		}

		err = CopyFile(srcPath, dstPath)
		PanicIfErr(err)
	}

	return
}

func Zip(src, dest, separator string) (out error) {
	defer RecoverPanic(func(e error) {
		out = e
	})

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
	defer RecoverPanic(func(e error) {
		out = e
	})

	r, err := zip.OpenReader(src)
	PanicIfErr(err)
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

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
