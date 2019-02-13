package dao

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockSubEntity struct {
	Entity
	Name string
	Age  int
}

func TestShouldCreateTable(t *testing.T) {
	assert := assert.New(t)

	dir, _ := ioutil.TempDir("", "t")
	defer os.RemoveAll(dir)

	dbPath := path.Join(dir, "test.db")

	client, err := NewInstance(dbPath)
	assert.Nil(err)
	assert.NotNil(client)

	entity := &MockSubEntity{
		Name: "yang",
		Age:  18,
	}
	client.Create(entity)
}
