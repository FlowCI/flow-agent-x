package domain

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestShouldParseString(t *testing.T) {
	assert := assert.New(t)

	volumes := NewVolumesFromString("name=1,dest=$HOME/ws,script=init.sh;name=2,dest=$HOME/ws1,script=init1.sh;")
	assert.Equal(2, len(volumes))

	v := volumes[0]
	assert.Equal("1", v.Name)
	assert.Equal("$HOME/ws", v.Dest)
	assert.Equal("init.sh", v.Script)

	v = volumes[1]
	assert.Equal("2", v.Name)
	assert.Equal("$HOME/ws1", v.Dest)
	assert.Equal("init1.sh", v.Script)
}