package fs_test

import (
	"math/rand"
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

	go9p.Serve("localhost:9999", staticFS.Server())
}

func ExampleDynamicFile() {
	dynamicFS := fs.NewFS("glenda", "glenda", 0555)
	dynamicFS.Root.AddChild(fs.NewDynamicFile(
		dynamicFS.NewStat("name.of.file", "owner.name", "group.name", 0444),
		func() []byte {
			return []byte(time.Now().String() + "\n")
		},
	))
	go9p.Serve("localhost:9999", dynamicFS.Server())
}

type randomFile struct {
	fs.BaseFile
}

func (f *randomFile) Read(fid uint32, offset, count uint64) ([]byte, error) {
	bs := make([]byte, count)
	rand.Read(bs)
	return bs, nil
}

// ExampleBaseFile demonstrates how to use a BaseFile to create a File type
// with custom behavior. In this case, we override the Read() function to
// return random bytes.
func ExampleBaseFile() {
	//	type randomFile struct {
	//		fs.BaseFile
	//	}
	//
	//	func (f *randomFile) Read(fid uint32, offset, count uint64) ([]byte, error) {
	//		bs := make([]byte, count)
	//		rand.Read(bs)
	//		return bs, nil
	//	}

	randomFS := fs.NewFS("glenda", "glenda", 0555)
	randomFS.Root.AddChild(&randomFile{
		*fs.NewBaseFile(randomFS.NewStat("random", "owner.name", "group.name", 0444)),
	})

	go9p.Serve("localhost:9999", randomFS.Server())
}

func ExampleRMFile() {
	staticFS := fs.NewFS("glenda", "glenda", 0555,
		// This Option will cause the FS to call fs.RMFile when a user with
		// permission attempts to remove a file. fs.RMFile simply deletes
		// the file with no further checking.
		fs.WithRemoveFile(fs.RMFile),
	)
	staticFS.Root.AddChild(fs.NewStaticFile(
		// Note the permissions 0544. Someone authenticated as the user "glenda"
		// will have the permission to remove the file.
		staticFS.NewStat("hello", "owner.name", "group.name", 0544),
		[]byte("Hello, World!\n"),
	))

	go9p.Serve("localhost:9999", staticFS.Server())
}

// ExampleWrappedFile demonstrates how to use a WrappedFile to create a File
// instance with custom behavior. In this case, we set ReadF to return random
// bytes.
func ExampleWrappedFile() {
	randomFS := fs.NewFS("glenda", "glenda", 0555)
	randomFS.Root.AddChild(fs.WrappedFile{
		File: fs.NewBaseFile(randomFS.NewStat("name.of.file", "owner.name", "group.name", 0444)),
		ReadF: func(fid uint64, offset uint64, count uint64) ([]byte, error) {
			bs := make([]byte, count)
			rand.Read(bs)
			return bs, nil
		},
	})

	go9p.Serve("localhost:9999", randomFS.Server())
}
