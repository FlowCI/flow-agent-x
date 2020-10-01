package domain

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestShouldParseString(t *testing.T) {
	assert := assert.New(t)

	v1 := "name=1,dest=$HOME/ws,script=init.sh,image=nginx:1,init=in-container-init.sh"
	v2 := "name=2,dest=$HOME/ws1,script=init1.sh,image=ubuntu:18.04,init=test.sh"

	volumes := NewVolumesFromString(fmt.Sprintf("%s;%s", v1, v2))
	assert.Equal(2, len(volumes))

	v := volumes[0]
	assert.Equal("1", v.Name)
	assert.Equal("$HOME/ws", v.Dest)
	assert.Equal("init.sh", v.Script)
	assert.Equal("nginx:1", v.Image)
	assert.Equal("in-container-init.sh", v.Init)

	v = volumes[1]
	assert.Equal("2", v.Name)
	assert.Equal("$HOME/ws1", v.Dest)
	assert.Equal("init1.sh", v.Script)
	assert.Equal("ubuntu:18.04", v.Image)
	assert.Equal("test.sh", v.Init)
}