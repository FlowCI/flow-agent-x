package dao

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestShouldGenQueryForCreateTable(t *testing.T) {
	assert := assert.New(t)

	entity := &MockSubEntity{
		ID:        "12345",
		Name:      "yang",
		Age:       18,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	query, err := toCreateQuery(entity)
	assert.Nil(err)

	expected := "create mock_sub_entity(id text not null primary key,name text,age integer);"
	assert.Equal(expected, query)
}
