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

	builder := &QueryBuilder{
		entity: entity,
	}

	query, err := builder.createQuery()
	assert.Nil(err)

	expected := "create mock_sub_entity(id text not null primary key,name text,age integer);"
	assert.Equal(expected, query)
}
