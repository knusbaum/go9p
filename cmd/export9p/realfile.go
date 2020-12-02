package main

import (
	"fmt"
	"hash/crc64"
	"io"
	"log"
	"os"
	"path"

	"github.com/knusbaum/go9p/fs"
	"github.com/knusbaum/go9p/proto"
)

type RealFile struct {
	Path  string
	opens map[uint64]*os.File
}

func NewRealFile(path string) *RealFile {
	return &RealFile{path, make(map[uint64]*os.File)}
}

func (f *RealFile) Parent() fs.Dir {
	if f.Path == "/" {
		return nil
	}
	return &RealDir{path.Dir(f.Path)}
}

func (f *RealFile) SetParent(d fs.Dir) {
	panic("THIS SHOULD NOT HAPPEN")
}

// func RealFileForPath(path string) (*RealFile, error) {
// 	info, err := os.Stat(path)
// 	if err != nil {
// 		return nil, fmt.Errorf("Failed to stat %s: %s", path, err)
// 	}
// 	user, group, err := getUserGroup(info)
// 	if err != nil {
// 		return nil, err
// 	}
// 	//f := &RealFile{BaseFile: *fs.NewBaseFile(exportFS.NewStat(info.Name(), user, group, uint32(info.Mode()))), Path: path, opens: make(map[uint64]*os.File)}
// 	return nil, fmt.Errorf("TODO")
// }

func (f *RealFile) Stat() proto.Stat {
	info, err := os.Stat(f.Path)
	if err != nil {
		log.Printf("Failed to stat %s: %s", f.Path, err)
		return proto.Stat{}
	}
	u, g, err := getUserGroup(info)
	if err != nil {
		u = "?"
		g = "?"
	}
	mode := uint32(info.Mode())
	return proto.Stat{
		Qid: proto.Qid{
			Qtype: uint8(mode >> 24),
			Vers:  uint32(info.ModTime().Unix()),
			Uid:   crc64.Checksum([]byte(f.Path), crc64Table),
		},
		Mode:   uint32(mode),
		Atime:  uint32(0),
		Mtime:  uint32(info.ModTime().Unix()),
		Length: uint64(info.Size()),
		Name:   info.Name(),
		Uid:    u,
		Gid:    g,
		Muid:   "",
	}
}

func (f *RealFile) WriteStat(s *proto.Stat) error {
	current := f.Stat()
	if s.Mode != current.Mode {
		fmt.Printf("OLD MODE: %#o NEW MODE: %#o\n", current.Mode, s.Mode)
		return os.Chmod(f.Path, os.FileMode(s.Mode))
		//return fmt.Errorf("mode change not implemented")
	}
	if s.Length != current.Length {
		return os.Truncate(f.Path, int64(s.Length))
	}
	if s.Name != current.Name {
		fmt.Printf("OLD NAME: %s NEW NAME: %s\n", current.Name, s.Name)
		//current.Name = s.Name
		dir := path.Dir(f.Path)
		newPath := path.Join(dir, s.Name)
		err := os.Rename(f.Path, newPath)
		if err != nil {
			return err
		}
		f.Path = newPath
		return nil
	}
	if s.Uid != current.Uid {
		fmt.Printf("OLD Uid: %s NEW Uid: %s\n", current.Uid, s.Uid)
		return fmt.Errorf("Owner change not implemented")
	}
	if s.Gid != current.Gid {
		fmt.Printf("OLD Gid: %s NEW Gid: %s\n", current.Gid, s.Gid)
		return fmt.Errorf("Group change not implemented")
	}
	//fmt.Printf("NEWSTAT: %#v\nOLDSTAT: %#v\n", s, current)
	// TODO
	//return fmt.Errorf("wstat not implemented")
	//fmt.Printf("NOTHING CHANGED.\n")
	return nil
}

func convertFlag(mode proto.Mode) int {
	var m int
	switch mode & 0x0F {
	case proto.Oread:
		m = os.O_RDONLY
	case proto.Owrite:
		m = os.O_WRONLY
	case proto.Ordwr:
		m = os.O_RDWR
	case proto.Oexec:
		m = os.O_RDONLY
	}
	if (mode & proto.Otrunc) > 0 {
		m |= os.O_TRUNC
	}
	return m
}

// Open always succeeds.
func (f *RealFile) Open(fid uint64, omode proto.Mode) error {
	file, err := os.OpenFile(f.Path, convertFlag(omode), 0)
	if err != nil {
		fmt.Printf("FAILED TO OPEN %s: %s\n", f.Path, err)
		return err
	}
	f.opens[fid] = file
	return nil
}

// Read always returns an empty slice.
func (f *RealFile) Read(fid uint64, offset uint64, count uint64) ([]byte, error) {
	file := f.opens[fid]
	bs := make([]byte, count)
	n, err := file.ReadAt(bs, int64(offset))
	if n > 0 {
		return bs[:n], nil
	}
	if err == io.EOF {
		return nil, nil
	}
	return nil, err
}

// Write always fails with an error.
func (f *RealFile) Write(fid uint64, offset uint64, data []byte) (uint32, error) {
	file := f.opens[fid]
	n, err := file.WriteAt(data, int64(offset))
	//fmt.Printf("[%s(%d)] %d/%d bytes at offset %d (%s)\n", f.Path, fid, n, len(data), offset, err)
	return uint32(n), err
}

// Close always succeeds.
func (f *RealFile) Close(fid uint64) error {
	file := f.opens[fid]
	delete(f.opens, fid)
	return file.Close()
}

// CreateRealFile is a function meant to be passed to WithCreateFile.
// It will add an empty StaticFile to the FS whenever a client attempts to
// create a file.
func CreateRealFile(filesystem *fs.FS, parent fs.Dir, user, name string, perm uint32, mode uint8) (fs.File, error) {
	fullPath := path.Join(fs.FullPath(parent), name)
	f, err := os.OpenFile(fullPath, os.O_CREATE, os.FileMode(perm))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return NewRealFile(fullPath), nil
}

func RemoveReal(filesystem *fs.FS, f fs.FSNode) error {
	return os.Remove(fs.FullPath(f))
}
