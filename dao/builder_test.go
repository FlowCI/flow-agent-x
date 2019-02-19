package dao

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestShouldBuildQueryForCreateTable(t *testing.T) {
	assert := assert.New(t)

	entity := &MockSubEntity{
		ID:        "12345",
		Name:      "yang",
		Age:       18,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	builder := initQueryBuilder(entity)
	query, err := builder.create()
	assert.Nil(err)

	expected := "CREATE TABLE mock_sub_entity (id TEXT NOT NULL PRIMARY KEY,name TEXT,age INTEGER);"
	assert.Equal(expected, query)
}
