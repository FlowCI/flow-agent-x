package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldToStringArray(t *testing.T) {
	assert := assert.New(t)

	variables := Variables{
		"hello": "world",
	}

	array := variables.ToStringArray()
	assert.NotNil(array)
	assert.Equal(1, len(array))
	assert.Equal("hello=world", array[0])
}
