package domain

import (
	"fmt"
	"os"
	"reflect"
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

func TestShouldDeepCopy(t *testing.T) {
	assert := assert.New(t)

	variables := Variables{"hello": "world"}
	copied := variables.Copy()
	assert.True(reflect.DeepEqual(variables, copied))
}

func TestShouldToStringArrayWithEnvVariables(t *testing.T) {
	assert := assert.New(t)

	variables := Variables{
		"SAY_HELLO": "${USER} hello",
	}

	variables.Resolve()
	array := variables.ToStringArray()
	assert.NotNil(array)
	assert.Equal(fmt.Sprintf("SAY_HELLO=%s hello", os.Getenv("USER")), array[0])
}

func TestShouldToStringArrayWithNestedEnvVariables(t *testing.T) {
	assert := assert.New(t)

	variables := Variables{
		"NESTED_HELLO": "${SAY_HELLO} hello",
		"SAY_HELLO": "${USER} hello",
	}

	variables.Resolve()
	array := variables.ToStringArray()
	assert.NotNil(array)
	assert.Equal(2, len(array))

	assert.Equal(fmt.Sprintf("NESTED_HELLO=%s hello hello", os.Getenv("USER")), array[0])
	assert.Equal(fmt.Sprintf("SAY_HELLO=%s hello", os.Getenv("USER")), array[1])
}

func TestShouldConnectVariables(t *testing.T) {
	assert := assert.New(t)

	varA := Variables{
		"NESTED_HELLO": "${SAY_HELLO_A} hello",
		"SAY_HELLO_A": "hello A",
	}

	varB := Variables{
		"SAY_HELLO_B": "hello B",
	}

	vars := ConnectVars(varA, varB)
	assert.NotNil(vars)
	assert.Equal(3, vars.Size())

	vars.Resolve()
	assert.Equal("hello A hello", vars["NESTED_HELLO"])
	assert.Equal("hello A", vars["SAY_HELLO_A"])
	assert.Equal("hello B", vars["SAY_HELLO_B"])
}
