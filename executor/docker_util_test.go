package executor

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestShouldTarAndUntarDir(t *testing.T) {
	assert := assert.New(t)

	dir := getTestDataDir()

	// tar flowid folder into reader
	reader, err := tarArchiveFromPath(filepath.Join(dir, "flowid"))
	assert.NoError(err)
	assert.NotNil(reader)

	// untar flowid folder into temp
	dest, err := ioutil.TempDir("", "test_tar_")
	assert.NoError(err)

	err = untarFromReader(reader, dest)
	assert.NoError(err)
}