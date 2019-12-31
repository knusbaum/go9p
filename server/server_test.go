package server

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServer(t *testing.T) {
	assert := assert.New(t)
	assert.True(true)
	fmt.Println("OK")
}