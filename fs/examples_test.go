package fs_test

import (
	"time"

	"github.com/knusbaum/go9p"
	"github.com/knusbaum/go9p/fs"
)

func ExampleStaticFile() {
	staticFS := fs.NewFS("glenda", "glenda", 0555)
	staticFS.Root.AddChild(fs.NewStaticFile(
		staticFS.NewStat("name.of.file", "owner.name", "group.name", 0444),
		[]byte("Hello, World!\n"),
	))

	go9p.PostSrv("example", staticFS.Server())
}

func ExampleDynamicFile() {
	dynamicFS := fs.NewFS("glenda", "glenda", 0555)
	dynamicFS.Root.AddChild(fs.NewDynamicFile(
		dynamicFS.NewStat("name.of.file", "owner.name", "group.name", 0444),
		func() []byte {
			return []byte(time.Now().String() + "\n")
		},
	))

	go9p.PostSrv("example", dynamicFS.Server())
}
