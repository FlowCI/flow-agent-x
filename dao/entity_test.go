package dao

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldParseStringToEntityField(t *testing.T) {
	assert := assert.New(t)

	field := parseEntityColumn("column=name")
	assert.NotNil(field)
	assert.Equal("name", field.Column)
}

func TestShouldPrimaryKeyNotNullForEntityField(t *testing.T) {
	assert := assert.New(t)

	field := &EntityColumn{
		Column:   "name",
		Pk:       true,
		Nullable: true,
		Type:     reflect.String,
	}

	_, err := field.toQuery()
	assert.NotNil(err)
	assert.Equal(ErrorPrimaryKeyCannotBeNull, err)
}

func TestShouldGenPrimaryKeyQueryForEntityField(t *testing.T) {
	assert := assert.New(t)

	field := &EntityColumn{
		Column: "name",
		Pk:     true,
		Type:   reflect.String,
	}

	q, err := field.toQuery()
	assert.Nil(err)
	assert.Equal("name TEXT NOT NULL PRIMARY KEY", q)
}

func TestShouldGenColumnQueryForEntityField(t *testing.T) {
	assert := assert.New(t)

	field := &EntityColumn{
		Column:   "name",
		Nullable: true,
		Type:     reflect.String,
	}

	q, err := field.toQuery()
	assert.Nil(err)
	assert.Equal("name TEXT", q)
}
