package main

import (
	"log"
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
	extendedFS, root := fs.NewFS("glenda", "glenda", 0777,
		fs.WithCreateFile(fs.CreateStaticFile),
		fs.WithCreateDir(fs.CreateStaticDir),
		fs.WithRemoveFile(fs.RMFile),
		//fs.WithAuth(fs.Plan9Auth),
		fs.WithAuth(fs.PlainAuth(map[string]string{
			"kyle": "foo",
			"jake": "bar",
		})),
	)

	root.AddChild(fs.NewStaticFile(
		extendedFS.NewStat("static", "glenda", "glenda", 0666),
		[]byte("Hello, World!\n"),
	))

	root.AddChild(fs.NewDynamicFile(
		extendedFS.NewStat("dynamic", "glenda", "glenda", 0666),
		func() []byte {
			return []byte(time.Now().String() + "\n")
		},
	))

	root.AddChild(&randomFile{
		*fs.NewBaseFile(extendedFS.NewStat("baseRand", "glenda", "glenda", 0666)),
	})

	root.AddChild(&fs.WrappedFile{
		File: fs.NewBaseFile(extendedFS.NewStat("wrappedRand", "glenda", "glenda", 0666)),
		ReadF: func(fid uint64, offset uint64, count uint64) ([]byte, error) {
			bs := make([]byte, count)
			rand.Read(bs)
			return bs, nil
		},
	})

	log.Println(go9p.Serve("0.0.0.0:9999", extendedFS.Server()))
}
