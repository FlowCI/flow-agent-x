package executor

import "os"

var binFiles = []*binFile{
	{
		name:       "wait-for-it.sh",
		content:    MustAsset("wait-for-it.sh"),
		permission: os.FileMode(0555),
	},
}

type binFile struct {
	name       string
	content    []byte
	permission os.FileMode
}
