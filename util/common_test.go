package util

import (
	"encoding/binary"
	"os/user"
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

func TestShouldParseStringWithEnvVariable(t *testing.T) {
	assert := assert.New(t)
	usr, _ := user.Current()

	assert.Equal("hello", ParseString("hello"))
	assert.Equal(usr.HomeDir+"/hello", ParseString("${HOME}/hello"))
	assert.Equal("/test"+usr.HomeDir+"/hello", ParseString("/test${HOME}/hello"))
	assert.Equal(usr.HomeDir+usr.HomeDir, ParseString("${HOME}${HOME}"))
}

func TestShouldDecodeUTF16(t *testing.T) {
	assert := assert.New(t)
	assert.Equal("-", UTF16BytesToString([]byte{0, 45}, binary.BigEndian))
}

func TestShouldTrimLeftBytes(t *testing.T) {
	assert := assert.New(t)
	assert.Equal([]byte{0, 1, 2}, BytesTrimLeft([]byte{0, 1, 2, 3, 4}, []byte{3, 4}))
}
