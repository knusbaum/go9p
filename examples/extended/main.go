package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/knusbaum/go9p"
	"github.com/knusbaum/go9p/fs"
)

type randomFile struct {
	fs.BaseFile
}

func (f *randomFile) Read(fid uint32, offset, count uint64) ([]byte, error) {
	bs := make([]byte, count)
	rand.Read(bs)
	return bs, nil
}

func main() {
	extendedFS := fs.NewFS("glenda", "glenda", 0555,
		fs.WithRemoveFile(fs.RMFile),
	)

	extendedFS.Root.AddChild(fs.NewStaticFile(
		extendedFS.NewStat("static", "glenda", "glenda", 0666),
		[]byte("Hello, World!\n"),
	))

	extendedFS.Root.AddChild(fs.NewDynamicFile(
		extendedFS.NewStat("dynamic", "glenda", "glenda", 0666),
		func() []byte {
			return []byte(time.Now().String() + "\n")
		},
	))

	extendedFS.Root.AddChild(&randomFile{
		*fs.NewBaseFile(extendedFS.NewStat("baseRand", "glenda", "glenda", 0666)),
	})

	extendedFS.Root.AddChild(&fs.WrappedFile{
		File: fs.NewBaseFile(extendedFS.NewStat("wrappedRand", "glenda", "glenda", 0666)),
		ReadF: func(fid uint64, offset uint64, count uint64) ([]byte, error) {
			bs := make([]byte, count)
			rand.Read(bs)
			return bs, nil
		},
	})

	fmt.Println(go9p.PostSrv("extendedFS", extendedFS.Server()))
}
