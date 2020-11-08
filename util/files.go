package util

import (
	"archive/zip"
	"io/ioutil"
	"os"
)

func IsFileExists(path string) bool {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return true
	}
	return false
}

func Zip(source, target, separator string) (out error) {
	defer func() {
		if r := recover(); r != nil {
			out = r.(error)
		}
	}()

	outFile, err := os.Create(target)
	PanicIfErr(err)

	defer outFile.Close()

	w := zip.NewWriter(outFile)
	addFiles(w, source, "", separator)

	err = w.Close()
	PanicIfErr(err)
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
