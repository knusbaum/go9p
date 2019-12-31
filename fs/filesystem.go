package fs

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/knusbaum/go9p/proto"
)

type FSNode interface {
	Stat() proto.Stat
	WriteStat(s *proto.Stat) error
	SetParent(d Dir)
	Parent() Dir
}

type File interface {
	FSNode
	Open(fid uint32, omode uint8) error
	Read(fid uint32, offset uint64, count uint64) ([]byte, error)
	Write(fid uint32, offset uint64, data []byte) (uint32, error)
	Close(fid uint32) error
}

type Dir interface {
	FSNode
	Children() map[string]FSNode
	AddChild(n FSNode) error
	DeleteChild(name string) error
}

func FullPath(f FSNode) string {
	if f == nil {
		return ""
	}
	parent := f.Parent()
	if parent == nil {
		fmt.Printf("ROOT: %s\n", f.Stat().Name)
		return f.Stat().Name
	}
	fp := FullPath(parent)
	fmt.Printf("Not root: %s / %s\n", fp, f.Stat().Name)
	return strings.Replace(fp+"/"+f.Stat().Name, "//", "/", -1)
}

type BaseFile struct {
	fStat  proto.Stat
	parent Dir
	sync.RWMutex
}

func NewBaseFile(s *proto.Stat) *BaseFile {
	return &BaseFile{fStat: *s}
}

func (f *BaseFile) Stat() proto.Stat {
	return f.fStat
}

func (f *BaseFile) WriteStat(s *proto.Stat) error {
	f.Lock()
	defer f.Unlock()
	f.fStat = *s
	return nil
}

func (f *BaseFile) SetParent(p Dir) {
	f.Lock()
	defer f.Unlock()
	f.parent = p
}

func (f *BaseFile) Parent() Dir {
	f.RLock()
	defer f.RUnlock()
	return f.parent
}

func (f *BaseFile) Open(fid uint32, omode uint8) error {
	return nil
}

func (f *BaseFile) Read(fid uint32, offset uint64, count uint64) ([]byte, error) {
	return []byte{}, nil
}

func (f *BaseFile) Write(fid uint32, offset uint64, data []byte) (uint32, error) {
	return 0, fmt.Errorf("Cannot write to file.")
}

func (f *BaseFile) Close(fid uint32) error {
	return nil
}

type FS struct {
	Root       Dir
	CreateFile func(fs *FS, parent Dir, user, name string, perm uint32, mode uint8) (File, error)
	CreateDir  func(fs *FS, parent Dir, user, name string, perm uint32, mode uint8) (Dir, error)
	WalkFail   func(fs *FS, parent Dir, name string) (FSNode, error)
	RemoveFile func(fs *FS, f FSNode) error
	uid        uint64
	sync.RWMutex
}

func NewFS(rootUser, rootGroup string, rootPerms uint32, opts ...Option) *FS {
	var fs FS
	d := NewStaticDir(fs.NewStat("/", rootUser, rootGroup, rootPerms|proto.DMDIR))
	fs.Root = d
	for _, o := range opts {
		o(&fs)
	}
	return &fs
}

func (fs *FS) NewQid(statMode uint32) proto.Qid {
	fs.Lock()
	defer fs.Unlock()
	uid := fs.uid
	fs.uid = fs.uid + 1
	return proto.Qid{
		Qtype: uint8(statMode >> 24),
		Vers:  0,
		Uid:   uid,
	}
}

func (fs *FS) NewStat(name, uid, gid string, mode uint32) *proto.Stat {
	return &proto.Stat{
		Type:   0,
		Dev:    0,
		Qid:    fs.NewQid(mode),
		Mode:   mode,
		Atime:  uint32(time.Now().Unix()),
		Mtime:  uint32(time.Now().Unix()),
		Length: 0,
		Name:   name,
		Uid:    uid,
		Gid:    gid,
		Muid:   uid,
	}
}

func CreateStaticFile(fs *FS, parent Dir, user, name string, perm uint32, mode uint8) (File, error) {
	f := NewStaticFile(fs.NewStat(name, user, user, perm), []byte{})
	parent.AddChild(f)
	return f, nil
}

func CreateStaticDir(fs *FS, parent Dir, user, name string, perm uint32, mode uint8) (Dir, error) {
	f := NewStaticDir(fs.NewStat(name, user, user, perm))
	parent.AddChild(f)
	return f, nil
}

func RMFile(fs *FS, f FSNode) error {
	parent := f.Parent()
	parent.DeleteChild(f.Stat().Name)
	return nil
}

type Option func(*FS)

func WithCreateFile(f func(fs *FS, parent Dir, user, name string, perm uint32, mode uint8) (File, error)) Option {
	return func(fs *FS) {
		fs.CreateFile = f
	}
}

func WithCreateDir(f func(fs *FS, parent Dir, user, name string, perm uint32, mode uint8) (Dir, error)) Option {
	return func(fs *FS) {
		fs.CreateDir = f
	}
}

func WithRemoveFile(f func(fs *FS, f FSNode) error) Option {
	return func(fs *FS) {
		fs.RemoveFile = f
	}
}

func WithWalkFailHandler(f func(fs *FS, parent Dir, name string) (FSNode, error)) Option {
	return func(fs *FS) {
		fs.WalkFail = f
	}
}
