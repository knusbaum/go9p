package fs

import (
	"testing"

	"github.com/knusbaum/go9p2/proto"

	"github.com/stretchr/testify/assert"
)

func TestStaticDir(t *testing.T) {
	assert := assert.New(t)
	var fs FS
	rootStat := fs.NewStat("", "user", "group", 0777|proto.DMDIR)
	root := NewStaticDir(rootStat)
	assert.Equal(*rootStat, root.Stat())
	rootStat.Uid = "TESTAROO"
	assert.NotEqual(*rootStat, root.Stat())
	err := root.WriteStat(rootStat)
	assert.NoError(err)
	assert.Equal(*rootStat, root.Stat())

	f := NewStaticFile(fs.NewStat("file", "user", "group", 0777), []byte("Hello, World!\n"))
	d := NewStaticDir(fs.NewStat("dir", "user", "group", 0777|proto.DMDIR))
	err = root.AddFile(f)
	assert.NoError(err)

	err = root.AddDir(d)
	assert.NoError(err)
	assert.Len(root.Children(), 2)
	assert.Equal(f, root.Children()["file"])
	assert.Equal(d, root.Children()["dir"])

	err = root.DeleteChild("file")
	assert.NoError(err)
	err = root.DeleteChild("dir")
	assert.NoError(err)
	assert.Len(root.Children(), 0)
}

func TestStaticFile(t *testing.T) {
	assert := assert.New(t)
	var fs FS

	fstat := fs.NewStat("file", "user", "group", 0777)
	f := NewStaticFile(fstat, []byte("Hello, World!\n"))

	assert.Equal(*fstat, f.Stat())
	fstat.Uid = "TESTAROO"
	assert.NotEqual(*fstat, f.Stat())
	err := f.WriteStat(fstat)
	assert.NoError(err)
	assert.Equal(*fstat, f.Stat())

	err = f.Open(0, proto.Ordwr)
	assert.NoError(err)

	r, err := f.Read(0, 0, 3)
	assert.NoError(err)
	assert.Equal(r, []byte("Hel"))

	r, err = f.Read(0, 3, 3)
	assert.NoError(err)
	assert.Equal(r, []byte("lo,"))

	r, err = f.Read(0, 0, 13)
	assert.NoError(err)
	assert.Equal(r, []byte("Hello, World!"))

	r, err = f.Read(0, 0, 100)
	assert.NoError(err)
	assert.Equal(r, []byte("Hello, World!\n"))

	r, err = f.Read(0, 1000000, 100)
	assert.NoError(err)
	assert.Equal(r, []byte(""))

	n, err := f.Write(0, 0, []byte("Goodbye"))
	assert.NoError(err)
	assert.Equal(n, uint32(7))
	r, err = f.Read(0, 0, 100)
	assert.NoError(err)
	assert.Equal(r, []byte("GoodbyeWorld!\n"))

	n, err = f.Write(0, 14, []byte("Hello Again.\n"))
	assert.NoError(err)
	assert.Equal(n, uint32(13))
	r, err = f.Read(0, 0, 100)
	assert.NoError(err)
	assert.Equal(r, []byte("GoodbyeWorld!\nHello Again.\n"))

	err = f.Close(0)
	assert.NoError(err)
}
