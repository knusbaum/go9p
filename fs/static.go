package fs

import (
	"sync"

	"github.com/knusbaum/go9p2/proto"
)

type StaticFile struct {
	BaseFile
	Data []byte
}

func NewStaticFile(s *proto.Stat, data []byte) *StaticFile {
	s.Length = uint64(len(data))
	return &StaticFile{
		BaseFile: BaseFile{fStat: *s},
		Data:     data,
	}
}

func (f *StaticFile) Stat() proto.Stat {
	f.Lock()
	defer f.Unlock()
	f.fStat.Length = uint64(len(f.Data))
	return f.fStat
}

func (f *StaticFile) Open(fid uint32, omode uint8) error {
	if omode&proto.Otrunc > 0 {
		f.Lock()
		defer f.Unlock()
		f.Data = make([]byte, 0)
	}
	return nil
}

func (f *StaticFile) Read(fid uint32, offset uint64, count uint64) ([]byte, error) {
	f.RLock()
	defer f.RUnlock()
	flen := uint64(len(f.Data))
	if offset >= flen {
		return []byte{}, nil
	}
	if offset+count > flen {
		count = flen - offset
	}
	return f.Data[offset : offset+count], nil
}

func (f *StaticFile) Write(fid uint32, offset uint64, data []byte) (uint32, error) {
	f.Lock()
	defer f.Unlock()
	flen := uint64(len(f.Data))
	count := uint64(len(data))
	if offset+count > flen {
		newlen := offset + count
		f.fStat.Length = newlen
		// TODO: Maybe this can be optimized
		f.Data = append(f.Data, make([]byte, newlen-flen)...)
	}

	copy(f.Data[offset:offset+count], data)
	return uint32(len(data)), nil
}

type StaticDir struct {
	dStat    proto.Stat
	children map[string]FSNode
	parent   Dir
	sync.RWMutex
}

func NewStaticDir(stat *proto.Stat) *StaticDir {
	dir := &StaticDir{
		dStat:    *stat,
		children: make(map[string]FSNode),
	}
	// Make sure stat is marked as a directory.
	dir.dStat.Mode |= proto.DMDIR
	// qtype bits should be consistent with Stat mode.
	dir.dStat.Qid.Qtype = uint8(dir.dStat.Mode >> 24)
	return dir
}

func (d *StaticDir) Stat() proto.Stat {
	return d.dStat
}

func (d *StaticDir) WriteStat(s *proto.Stat) error {
	d.Lock()
	defer d.Unlock()
	d.dStat = *s
	return nil
}

func (d *StaticDir) SetParent(p Dir) {
	d.Lock()
	defer d.Unlock()
	d.parent = p
}

func (d *StaticDir) Parent() Dir {
	d.RLock()
	defer d.RUnlock()
	return d.parent
}

func (d *StaticDir) Children() map[string]FSNode {
	d.RLock()
	defer d.RUnlock()
	ret := make(map[string]FSNode)
	for k, v := range d.children {
		ret[k] = v
	}
	return ret
}

func (d *StaticDir) AddChild(n FSNode) error {
	d.Lock()
	defer d.Unlock()
	d.children[n.Stat().Name] = n
	n.SetParent(d)
	return nil
}

func (d *StaticDir) DeleteChild(name string) error {
	d.Lock()
	defer d.Unlock()
	c := d.children[name]
	c.SetParent(nil)
	delete(d.children, name)
	return nil
}
