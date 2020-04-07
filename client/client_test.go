package client

import (
	"fmt"
	"io"
	"testing"
	"time"
	"log"

	"github.com/knusbaum/go9p"
	"github.com/knusbaum/go9p/fs"

	"github.com/stretchr/testify/assert"
)

type TwoPipe struct {
	*io.PipeReader
	*io.PipeWriter
}

func (t *TwoPipe) Close() error {
	t.PipeReader.Close()
	t.PipeWriter.Close()
	return nil
}

var helloText string = "Hello, World!"

func setup(t *testing.T) (*fs.FS, *Client) {
	testFS := fs.NewFS("glenda", "glenda", 0777)
	hello := fs.NewStaticFile(testFS.NewStat("hello", "glenda", "glenda", 0444), []byte(helloText))
	testFS.Root.AddChild(hello)

	p1r, p1w := io.Pipe()
	p2r, p2w := io.Pipe()
	go go9p.ServeReadWriter(p1r, p2w, testFS.Server())

	c, err := NewClient(&TwoPipe{p2r, p1w}, "glenda", "")
	fmt.Printf("C: %#v, ERR: %#v\n", c, err)
	//assert.NoError(t, err)

	return testFS, c
}

func TestShutdown(testingtttt *testing.T) {
	testFS := fs.NewFS("glenda", "glenda", 0777)
	hello := fs.NewStaticFile(testFS.NewStat("hello", "glenda", "glenda", 0444), []byte(helloText))
	testFS.Root.AddChild(hello)

	p1r, p1w := io.Pipe()
	p2r, p2w := io.Pipe()
	go func() { err := go9p.ServeReadWriter(p1r, p2w, testFS.Server()); log.Printf("ERROR RESULT: %s", err) }()
	NewClient(&TwoPipe{p2r, p1w}, "glenda", "")
	
	p1r.Close()
	p2r.Close()
	time.Sleep(5 * time.Second)
}

func TestWalk(t *testing.T) {
	tfs, _ := setup(t)

	p1r, p1w := io.Pipe()
	p2r, p2w := io.Pipe()
	go go9p.ServeReadWriter(p1r, p2w, tfs.Server())

	c, err := NewClient(&TwoPipe{p2r, p1w}, "glenda", "")
	fmt.Printf("C: %#v, ERR: %#v\n", c, err)
	assert.NoError(t, err)
	f, err := c.Open("/foo/bar/baz/hello")
	assert.Error(t, err)

	f, err = c.Open("/hello")
	assert.NoError(t, err)

	bs := make([]byte, 1024)
	n, err := f.Read(bs)
	assert.NoError(t, err)
	assert.Equal(t, helloText, string(bs[:n]))

	err = f.Close()
	assert.NoError(t, err)
}
