package dao

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldParseStringToEntityField(t *testing.T) {
	assert := assert.New(t)

	field := parseEntityField("column=name")
	assert.NotNil(field)
	assert.Equal("name", field.Column)
}

func TestShouldPrimaryKeyNotNullForEntityField(t *testing.T) {
	assert := assert.New(t)

	field := &EntityField{
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

	field := &EntityField{
		Column: "name",
		Pk:     true,
		Type:   reflect.String,
	}

	q, err := field.toQuery()
	assert.Nil(err)
	assert.Equal("name text not null primary key", q)
}

func TestShouldGenColumnQueryForEntityField(t *testing.T) {
	assert := assert.New(t)

	field := &EntityField{
		Column:   "name",
		Nullable: true,
		Type:     reflect.String,
	}

	q, err := field.toQuery()
	assert.Nil(err)
	assert.Equal("name text", q)
}
