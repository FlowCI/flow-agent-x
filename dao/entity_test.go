package dao

import (
	"testing"

	"github.com/flowci/flow-agent-x/util"

	"github.com/stretchr/testify/assert"
)

func TestShouldPrimaryKeyNotNullForEntityField(t *testing.T) {
	assert := assert.New(t)
	nameField, _ := util.GetType(MockSubEntity{}).FieldByName("Name")

	field := &EntityColumn{
		Field:    nameField,
		Column:   "name",
		Pk:       true,
		Nullable: true,
	}

	_, err := field.toQuery()
	assert.NotNil(err)
	assert.Equal(ErrorPrimaryKeyCannotBeNull, err)
}

func TestShouldGenPrimaryKeyQueryForEntityField(t *testing.T) {
	assert := assert.New(t)
	nameField, _ := util.GetType(MockSubEntity{}).FieldByName("Name")

	field := &EntityColumn{
		Field:  nameField,
		Column: "name",
		Pk:     true,
	}

	q, err := field.toQuery()
	assert.Nil(err)
	assert.Equal("name TEXT NOT NULL PRIMARY KEY", q)
}

func TestShouldGenColumnQueryForEntityField(t *testing.T) {
	assert := assert.New(t)
	nameField, _ := util.GetType(MockSubEntity{}).FieldByName("Name")

	field := &EntityColumn{
		Field:    nameField,
		Column:   "name",
		Nullable: true,
	}

	q, err := field.toQuery()
	assert.Nil(err)
	assert.Equal("name TEXT", q)
}
