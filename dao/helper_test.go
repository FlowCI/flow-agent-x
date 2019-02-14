package dao

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldFlatCamelString(t *testing.T) {
	assert := assert.New(t)

	str := FlatCamelString("MockSuperEntity")
	assert.Equal("mock_super_entity", str)
}

func TestShouldCapitalFirstChar(t *testing.T) {
	assert := assert.New(t)

	str := CapitalFirstChar("column")
	assert.Equal("Column", str)
}
