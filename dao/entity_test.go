package dao

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldParseStringToEntityField(t *testing.T) {
	assert := assert.New(t)

	field := ParseEntityField("column=name")
	assert.NotNil(field)
	assert.Equal("name", field.Column)
}
