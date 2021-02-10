package api

import (
	"encoding/base64"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestShouldEncodeCacheName(t *testing.T) {
	assert := assert.New(t)

	encoded := encodeCacheName("/ws", "/ws/a/b/c")
	assert.Equal(base64.StdEncoding.EncodeToString([]byte("a/b/c")), encoded)

	encoded = encodeCacheName("/ws", "/ws/test.log")
	assert.Equal(base64.StdEncoding.EncodeToString([]byte("test.log")), encoded)
}

func TestShouldDecodeCacheName(t *testing.T) {
	assert := assert.New(t)

	decoded := decodeCacheName("YS9iL2M=")
	assert.Equal("a/b/c", decoded)

	decoded = decodeCacheName("dGVzdC5sb2c=")
	assert.Equal("test.log", decoded)
}
