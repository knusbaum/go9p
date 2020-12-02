// This is a sample filesystem that serves a couple "utilities"
// There's /time, which when read, will return a human-readable
// string of the current time.
// There's also /random, which is a file of infinite-length
// containing random bytes.
// Finally, there's /events, which records all of the high-level
// callbacks invoked on the Server struct.
package main

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/knusbaum/go9p"
	"github.com/knusbaum/go9p/fs"
	"github.com/knusbaum/go9p/proto"
)

func addEvent(f *fs.StaticFile, s string) {
	f.Lock()
	defer f.Unlock()
	f.Data = append(f.Data, []byte(s+"\n")...)
}

func WrapEvents(evFile *fs.StaticFile, f fs.File) fs.File {
	fname := f.Stat().Name
	return &fs.WrappedFile{
		File: f,
		OpenF: func(fid uint64, omode proto.Mode) error {
			addEvent(evFile, fmt.Sprintf("Open %s: mode: %d", fname, omode))
			return f.Open(fid, omode)
		},
		ReadF: func(fid uint64, offset uint64, count uint64) ([]byte, error) {
			addEvent(evFile, fmt.Sprintf("Read %s: offset %d, count %d", fname, offset, count))
			return f.Read(fid, offset, count)
		},
		WriteF: func(fid uint64, offset uint64, data []byte) (uint32, error) {
			addEvent(evFile, fmt.Sprintf("Write %s: offset %d, data %d bytes", fname, offset, len(data)))
			return f.Write(fid, offset, data)
		},
		CloseF: func(fid uint64) error {
			addEvent(evFile, fmt.Sprintf("Close %s", fname))
			return f.Close(fid)
		},
	}
}

func main() {
	utilFS, root := fs.NewFS("glenda", "glenda", 0777)
	events := fs.NewStaticFile(utilFS.NewStat("events", "glenda", "glenda", 0444), []byte{})
	root.AddChild(events)
	root.AddChild(
		WrapEvents(events, fs.NewDynamicFile(utilFS.NewStat("time", "glenda", "glenda", 0444),
			func() []byte {
				return []byte(time.Now().String() + "\n")
			},
		)),
	)
	root.AddChild(
		WrapEvents(events, &fs.WrappedFile{
			File: fs.NewBaseFile(utilFS.NewStat("random", "glenda", "glenda", 0444)),
			ReadF: func(fid uint64, offset uint64, count uint64) ([]byte, error) {
				bs := make([]byte, count)
				rand.Reader.Read(bs)
				return bs, nil
			},
		}),
	)
	// Post a local service.
	go9p.PostSrv("utilfs", utilFS.Server())
}
