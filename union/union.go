package union

import (
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"strings"
	"sync"

	"github.com/knusbaum/go9p/client"
	"github.com/knusbaum/go9p/fs"
	"github.com/knusbaum/go9p/proto"
)

type MountOption int

const (
	REPLACE MountOption = iota
	BEFORE
	AFTER
)

type mountEntry struct {
	c          *client.Client
	f          *UnionFile
	d          *UnionDir
	mountPoint string
	replace    bool
	create     bool
}

type baseUnionNode struct {
	sync.RWMutex
	parent *UnionDir
	path   string
	mount  mountEntry
}

func (n *baseUnionNode) Parent() fs.Dir {
	n.RLock()
	defer n.RUnlock()
	return n.parent
}

func (n *baseUnionNode) SetParent(p fs.Dir) {
	n.Lock()
	defer n.Unlock()

	ud, ok := p.(*UnionDir)
	if !ok {
		panic(fmt.Errorf("parent must be set to a union directory"))
	}
	n.parent = ud
}

func (n *baseUnionNode) Stat() proto.Stat {
	// This is the root
	if n.path == "" {
		// TODO make this stat as the root directory
		return proto.Stat{
			Mode: 0x80000000,
		}
	}

	rel, err := filepath.Rel(n.mount.mountPoint, n.path)
	if err != nil {
		return proto.Stat{}
	}

	switch {
	case n.mount.c != nil:
		stat, err := n.mount.c.Stat(rel)
		if err != nil {
			return proto.Stat{}
		}
		return *stat
	case n.mount.f != nil:
		return n.mount.f.Stat()
	case n.mount.d != nil:
		return n.mount.d.Stat()
	}

	panic(fmt.Errorf("invalid mount table state"))
}

func (n *baseUnionNode) WriteStat(s *proto.Stat) error {
	if n.mount.c == nil {
		return fmt.Errorf("the root directory cannot be modified")
	}

	rel, err := filepath.Rel(n.mount.mountPoint, n.path)
	if err != nil {
		// TODO consider panic instead
		return fmt.Errorf("this should never happen")
	}

	switch {
	case n.mount.c != nil:
		return n.mount.c.WStat(rel, s)
	case n.mount.f != nil:
		return n.mount.f.WriteStat(s)
	case n.mount.d != nil:
		on := n.mount.d.find(rel)
		if on == nil {
			return fmt.Errorf("stale mount")
		}
		return on.WriteStat(s)
	}

	panic(fmt.Errorf("invalid mount table state"))
}

type UnionDir struct {
	baseUnionNode
	mountTable []mountEntry
}

// Create a new union directory.
//
// Note that the caller must take a copy of the mount table
// so that it can be owned by the new union directory.
func NewUnionDir(n baseUnionNode, mountTable []mountEntry) *UnionDir {
	return &UnionDir{
		baseUnionNode: n,
		mountTable:    mountTable,
	}
}

func (ud *UnionDir) find(rel string) *baseUnionNode {
	if strings.HasPrefix(rel, "/") {
		rel = rel[1:]
	}

	if rel == "" {
		return &ud.baseUnionNode
	}

	parts := strings.Split(rel, "/")
	d := ud
	var n *baseUnionNode

	for _, part := range parts {
		if d == nil {
			return nil
		}

		children := d.Children()
		child, ok := children[part]
		if !ok {
			return nil
		}

		n = child.(*baseUnionNode)
		if cd, ok := child.(*UnionDir); ok {
			d = cd
		}
	}

	return n
}

func (ud *UnionDir) findDir(rel string) *UnionDir {
	if strings.HasPrefix(rel, "/") {
		rel = rel[1:]
	}

	if rel == "" {
		return ud
	}

	parts := strings.Split(rel, "/")
	d := ud
	var n *UnionDir

	for _, part := range parts {
		if d == nil {
			return nil
		}

		children := d.Children()
		child, ok := children[part]
		if !ok {
			return nil
		}

		n, ok = child.(*UnionDir)
		d = n
		if !ok {
			return nil
		}
	}

	return n
}

func (ud *UnionDir) findFile(rel string) *UnionFile {
	if strings.HasPrefix(rel, "/") {
		rel = rel[1:]
	}

	if rel == "" {
		return nil
	}

	parts := strings.Split(rel, "/")
	d := ud
	var n fs.FSNode

	for _, part := range parts {
		if d == nil {
			return nil
		}

		children := d.Children()
		child, ok := children[part]
		if !ok {
			return nil
		}

		n = child
		if cd, ok := child.(*UnionDir); ok {
			d = cd
		}
	}

	if uf, ok := n.(*UnionFile); ok {
		return uf
	}

	return nil
}

func (ud *UnionDir) RemoveFile(f fs.FSNode) error {
	// TODO maybe someone will want to remove directories someday
	uf, ok := f.(*UnionFile)
	if !ok {
		return fmt.Errorf("cannot remove file that is not a union filesystem file")
	}

	rel, err := filepath.Rel(ud.mount.mountPoint, uf.path)
	if err != nil {
		return err
	}

	switch {
	case ud.mount.c != nil:
		return ud.mount.c.Remove(rel)
	case ud.mount.d != nil:
		on := ud.mount.d.find(rel)
		if on == nil {
			return fmt.Errorf("stale mount")
		}
		if on.parent == nil {
			return fmt.Errorf("cannot remove root")
		}
		return on.parent.RemoveFile(on)
	}

	panic(fmt.Errorf("invalid mount table state"))
}

func (ud *UnionDir) CreateFile(user, name string, perm uint32, mode uint8) (fs.File, error) {
	// First, we find the mount that will permit creation
	mte := ud.mount
	if !mte.create {
		for _, me := range ud.mountTable {
			if me.create == true && ud.path == me.mountPoint {
				mte = me
				break
			}
		}
	}

	if !mte.create {
		return nil, fmt.Errorf("creation is not permitted here")
	}

	rel, err := filepath.Rel(mte.mountPoint, filepath.Join(ud.path, name))
	if err != nil {
		return nil, err
	}

	switch {
	case mte.c != nil:
		// TODO how do we pass in the user here?
		f, err := mte.c.Create(rel, iofs.FileMode((uint32(mode)<<24)|(perm&0x00FFFFFF)))
		if err != nil {
			return nil, err
		}
		// The file comes pre-opened on create using the client API
		// so we close it for now.
		f.Close()

		n := baseUnionNode{
			path:  filepath.Join(ud.path, name),
			mount: mte,
		}

		return NewUnionFile(n), nil
	case mte.d != nil:
		on := mte.d.find(rel)
		if on == nil {
			return nil, fmt.Errorf("stale mount")
		}
		if on.parent == nil {
			return nil, fmt.Errorf("cannot create root")
		}
		return on.parent.CreateFile(user, name, perm, mode)
	}

	panic(fmt.Errorf("invalid mount table state"))
}

func (ud *UnionDir) CreateDir(user, name string, perm uint32, mode uint8) (fs.Dir, error) {
	// TODO check the mode to ensure that this is DMDIR

	mte := ud.mount
	if !mte.create {
		for _, me := range ud.mountTable {
			if me.create == true && ud.path == me.mountPoint {
				mte = me
				break
			}
		}
	}

	if !mte.create {
		return nil, fmt.Errorf("creation is not permitted here")
	}

	rel, err := filepath.Rel(mte.mountPoint, filepath.Join(ud.path, name))
	if err != nil {
		return nil, err
	}

	switch {
	case mte.c != nil:
		// TODO how do we pass in the user here?
		f, err := mte.c.Create(rel, iofs.FileMode((uint32(mode)<<24)|(perm&0x00FFFFFF)))
		if err != nil {
			return nil, err
		}
		// The file comes pre-opened on create using the client API
		// so we close it for now.
		f.Close()

		n := baseUnionNode{
			path:  filepath.Join(ud.path, name),
			mount: mte,
		}

		// Lock to grab a copy of the mount table
		ud.RLock()
		mountTable := append([]mountEntry{}, ud.mountTable...)
		ud.RUnlock()
		return NewUnionDir(n, mountTable), nil
	case mte.d != nil:
		on := mte.d.find(rel)
		if on == nil {
			return nil, fmt.Errorf("stale mount")
		}
		if on.parent == nil {
			return nil, fmt.Errorf("cannot create the root")
		}
		return on.parent.CreateDir(user, name, perm, mode)

	}

	panic(fmt.Errorf("invalid mount table state"))
}

func (ud *UnionDir) Children() map[string]fs.FSNode {
	children := make(map[string]fs.FSNode)

	// Lock to read the mount table to take a copy of it
	ud.RLock()
	mountTable := append([]mountEntry{}, ud.mountTable...)
	ud.RUnlock()

	// TODO consider a scatter/gather approach with goroutines since these can be I/O blocking
	for _, me := range mountTable {
		isCurrentMount := ud.mount.c == me.c && ud.mount.d == me.d && ud.mount.f == me.f

		if ud.path != me.mountPoint && !isCurrentMount {
			continue
		}

		rel, err := filepath.Rel(me.mountPoint, ud.path)
		if err != nil {
			continue
		}

		if rel == "." {
			rel = "/"
		}

		switch {
		case me.c != nil:
			sts, err := me.c.Readdir(rel)
			// TODO should we expire this mount somehow?
			if err != nil {
				continue
			}

			for _, stat := range sts {
				n := baseUnionNode{
					path:  filepath.Join(ud.path, stat.Name),
					mount: me,
				}

				// TODO this constant should really be in the go9p package
				if stat.Mode&0x80000000 != 0 {
					children[stat.Name] = NewUnionDir(n, append([]mountEntry{}, mountTable...))
				} else {
					// TODO check if there is a mount point for the file here
					children[stat.Name] = NewUnionFile(n)
				}
			}
		case me.d != nil:
			on := me.d.findDir(rel)
			if on == nil {
				continue
			}
			subchildren := on.Children()
			for name, n := range subchildren {
				if udn, ok := n.(*UnionDir); ok {
					udn.mount = me
					udn.path = filepath.Join(ud.path, name)
					udn.mountTable = append([]mountEntry{}, mountTable...)
					children[name] = udn
				}
				if ufn, ok := n.(*UnionFile); ok {
					ufn.mount = me
					ufn.path = filepath.Join(ud.path, name)
					children[name] = ufn
				}
			}
		}

		if me.replace && !isCurrentMount {
			return children
		}
	}

	return children
}

type UnionFile struct {
	baseUnionNode
	opens   map[uint64]*client.File
	openufs map[uint64]*UnionFile
}

func NewUnionFile(n baseUnionNode) *UnionFile {
	return &UnionFile{
		baseUnionNode: n,
		opens:         make(map[uint64]*client.File),
		openufs:       make(map[uint64]*UnionFile),
	}
}

func (uf *UnionFile) Open(fid uint64, omode proto.Mode) error {
	rel, err := filepath.Rel(uf.mount.mountPoint, uf.path)
	if err != nil {
		return err
	}

	uf.Lock()
	defer uf.Unlock()

	switch {
	case uf.mount.c != nil:
		f, err := uf.mount.c.Open(rel, omode)
		if err != nil {
			return err
		}
		uf.opens[fid] = f
		return nil
	case uf.mount.d != nil:
		// Store the relative path for later when we're dealing with only fids
		on := uf.mount.d.findFile(rel)
		if on == nil {
			return fmt.Errorf("stale mount")
		}
		uf.openufs[fid] = on
		return on.Open(fid, omode)
	case uf.mount.f != nil:
		return uf.mount.f.Open(fid, omode)
	}

	panic(fmt.Errorf("invalid mount table state"))
}

func (uf *UnionFile) Read(fid uint64, offset uint64, count uint64) ([]byte, error) {
	uf.RLock()
	defer uf.RUnlock()

	switch {
	case uf.mount.c != nil:
		file, ok := uf.opens[fid]
		if !ok {
			return []byte{}, fmt.Errorf("fid is not open: %d", fid)
		}

		buf := make([]byte, count)
		// TODO how to make this type conversion to signed safe
		n, err := file.ReadAt(buf, int64(offset))
		return buf[:n], err
	case uf.mount.d != nil:
		on := uf.openufs[fid]
		if on == nil {
			return []byte{}, fmt.Errorf("unknown fid, or file closed")
		}
		return on.Read(fid, offset, count)
	case uf.mount.f != nil:
		return uf.mount.f.Read(fid, offset, count)
	}

	panic(fmt.Errorf("invalid mount table state"))
}

func (uf *UnionFile) Write(fid uint64, offset uint64, data []byte) (uint32, error) {
	uf.RLock()
	defer uf.RUnlock()

	switch {
	case uf.mount.c != nil:
		file, ok := uf.opens[fid]
		if !ok {
			return 0, fmt.Errorf("fid is not open: %d", fid)
		}

		// TODO how to make this type conversion to signed safe
		n, err := file.WriteAt(data, int64(offset))
		return uint32(n), err
	case uf.mount.d != nil:
		on := uf.openufs[fid]
		if on == nil {
			return 0, fmt.Errorf("unknown fid, or file closed")
		}
		return on.Write(fid, offset, data)
	case uf.mount.f != nil:
		return uf.mount.f.Write(fid, offset, data)
	}

	panic(fmt.Errorf("invalid mount table state"))
}

func (uf *UnionFile) Close(fid uint64) error {
	switch {
	case uf.mount.c != nil:
		file, ok := uf.opens[fid]
		if !ok {
			return fmt.Errorf("fid is not open: %d", fid)
		}
		delete(uf.opens, fid)
		return file.Close()
	case uf.mount.f != nil:
		on := uf.openufs[fid]
		if on == nil {
			return fmt.Errorf("unknown fid, or file closed")
		}
		delete(uf.openufs, fid)
		return on.Close(fid)
	case uf.mount.f != nil:
		return uf.mount.f.Close(fid)
	}

	panic(fmt.Errorf("invalid mount table state"))
}

func CreateUnionFile(fs *fs.FS, p fs.Dir, user, name string, perm uint32, mode uint8) (fs.File, error) {
	parent, ok := p.(*UnionDir)
	if !ok {
		return nil, fmt.Errorf("directory is not a union filesystem directory")
	}
	return parent.CreateFile(user, name, perm, mode)
}

func CreateUnionDir(fs *fs.FS, p fs.Dir, user, name string, perm uint32, mode uint8) (fs.Dir, error) {
	parent, ok := p.(*UnionDir)
	if !ok {
		return nil, fmt.Errorf("directory is not a union filesystem directory")
	}

	return parent.CreateDir(user, name, perm, mode)
}

func RemoveUnionFile(fs *fs.FS, f fs.FSNode) error {
	parent, ok := f.Parent().(*UnionDir)
	if !ok {
		return fmt.Errorf("parent is not a union filesystem directory")
	}

	return parent.RemoveFile(f)
}

func NewUnionFS() *fs.FS {
	return &fs.FS{
		Root:       &UnionDir{baseUnionNode: baseUnionNode{path: "/"}},
		CreateFile: CreateUnionFile,
		CreateDir:  CreateUnionDir,
		RemoveFile: RemoveUnionFile,
	}
}

// Mount a 9p client into the filesystem
func Mount(fs *fs.FS, c *client.Client, old string, option MountOption, create bool) error {
	root, ok := fs.Root.(*UnionDir)
	if !ok {
		return fmt.Errorf("cannot mount into non-union filesystem.")
	}

	entry := mountEntry{
		c:          c,
		mountPoint: old,
		replace:    option == REPLACE,
		create:     create,
	}

	root.Lock()
	defer root.Unlock()

	if option == BEFORE || option == REPLACE {
		root.mountTable = append([]mountEntry{entry}, root.mountTable...)
	} else {
		root.mountTable = append(root.mountTable, entry)
	}

	return nil
}

// Bind a part of the filesystem to another part
func Bind(fs *fs.FS, path string, old string, option MountOption, create bool) error {
	root, ok := fs.Root.(*UnionDir)
	if !ok {
		return fmt.Errorf("cannot bind into non-union filesystem")
	}

	// TODO handle mounting on files
	pathdir := root.findDir(path)
	if pathdir == nil {
		return fmt.Errorf("cannot find path to bind")
	}

	entry := mountEntry{
		d:          pathdir,
		mountPoint: old,
		replace:    option == REPLACE,
		create:     create,
	}

	root.Lock()
	defer root.Unlock()

	if option == BEFORE || option == REPLACE {
		root.mountTable = append([]mountEntry{entry}, root.mountTable...)
	} else {
		root.mountTable = append(root.mountTable, entry)
	}

	return nil
}

// Unmount everything that is mounted at the provided path.
//
// 9p clients that have been unmounted are returned.
func UnmountPoint(fs *fs.FS, old string) ([]*client.Client, error) {
	root, ok := fs.Root.(*UnionDir)
	if !ok {
		return []*client.Client{}, fmt.Errorf("cannot unmount in a non-union filesystem")
	}

	root.Lock()
	defer root.Unlock()

	clients := []*client.Client{}
	mountTable := []mountEntry{}

	for _, me := range root.mountTable {
		if me.mountPoint != old {
			mountTable = append(mountTable, me)
		} else if me.c != nil {
			clients = append(clients, me.c)
		}
	}

	root.mountTable = mountTable

	return clients, nil
}

// Unmount the client that was previously mounted at the provided path.
func UnmountClient(fs *fs.FS, c *client.Client, old string) error {
	root, ok := fs.Root.(*UnionDir)
	if !ok {
		return fmt.Errorf("cannot unmount in a non-union filesystem")
	}

	root.Lock()
	defer root.Unlock()

	mountTable := []mountEntry{}

	for _, me := range root.mountTable {
		if me.mountPoint != old || me.c != c {
			mountTable = append(mountTable, me)
		}
	}

	root.mountTable = mountTable

	return nil
}

// Unmount the bind that was previously set at the provided path.
func UnmountBind(fs *fs.FS, path string, old string) error {
	root, ok := fs.Root.(*UnionDir)
	if !ok {
		return fmt.Errorf("cannot unmount in a non-union filesystem")
	}

	root.Lock()
	defer root.Unlock()

	mountTable := []mountEntry{}

	for _, me := range root.mountTable {
		// TODO handle file level binds
		if me.mountPoint != old || me.d == nil || me.d.path != path {
			mountTable = append(mountTable, me)
		}
	}

	root.mountTable = mountTable

	return nil
}
