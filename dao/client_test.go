package dao

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldCreateTable(t *testing.T) {
	assert := assert.New(t)

	dir, _ := ioutil.TempDir("", "t")
	defer os.RemoveAll(dir)

	// dbPath := path.Join(dir, "test.db")

	client, err := NewInstance("/Users/yang/test.db")
	assert.Nil(err)
	assert.NotNil(client)

	entity := &MockSubEntity{}
	client.Create(entity)
}
