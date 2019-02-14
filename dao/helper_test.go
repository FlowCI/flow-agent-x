package dao

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldFlatCamelString(t *testing.T) {
	assert := assert.New(t)

	str := flatCamelString("MockSuperEntity")
	assert.Equal("mock_super_entity", str)
}

func TestShouldCapitalFirstChar(t *testing.T) {
	assert := assert.New(t)

	str := capitalFirstChar("column")
	assert.Equal("Column", str)
}
