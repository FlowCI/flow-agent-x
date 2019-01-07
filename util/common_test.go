package util

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestType struct {
	Name string
}

func TestShouldDetectPointerType(t *testing.T) {
	assert := assert.New(t)

	p := &TestType{}
	assert.True(IsPointerType(p))
}

func TestShouldGetTypeOfPointer(t *testing.T) {
	assert := assert.New(t)

	p := new(TestType)
	assert.Equal(reflect.TypeOf(TestType{}), GetType(p))
}
