package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"hash/crc64"
	"io"
	"log"
	"math"
	"os"
	"os/user"
	"path"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/knusbaum/go9p"
	"github.com/knusbaum/go9p/cert"
	"github.com/knusbaum/go9p/client"
	"github.com/knusbaum/go9p/proto"

	fans "9fans.net/go/plan9/client"
)

var crc64Table = crc64.MakeTable(0xC96C5795D7870F42)

//var DefaultTTL = 10 * time.Second
//var ncTTL = uint64(10)

var CacheTTL = 0 * time.Second
var ncTTL = uint64(0)

var dirCacheLock sync.RWMutex
var dirCache map[string]*Dir = make(map[string]*Dir)

var unknownUID uint32
var unknownGID uint32
var authUser string
var sysUser, sysGroup uint32

func uidForUser(uid string) uint32 {
	if uid == authUser {
		return sysUser
	}
	return unknownUID
}

func gidForGroup(gid string) uint32 {
	if gid == authUser {
		return sysGroup
	}
	return unknownGID
}

func dirGet(path string) *Dir {
	dirCacheLock.RLock()
	defer dirCacheLock.RUnlock()
	return dirCache[path]
}

func dirPut(path string, d *Dir) {
	dirCacheLock.Lock()
	defer dirCacheLock.Unlock()
	dirCache[path] = d
}

type Dir struct {
	fs.Inode
	client *client.Client
	path   string

	statCache *proto.Stat
	statTTL   time.Time

	dirCache []proto.Stat
	dirTTL   time.Time
}

type StatDir struct {
	Dir
	Mode uint32
}

func (r *StatDir) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	//log.Printf("(*StatDir).Getattr(%s)", r.path)
	e := r.Dir.Getattr(ctx, f, out)
	out.Mode = r.Mode
	return e
}

var _ = (fs.NodeLookuper)((*Dir)(nil))
var _ = (fs.NodeReaddirer)((*Dir)(nil))
var _ = (fs.NodeCreater)((*Dir)(nil))
var _ = (fs.NodeGetattrer)((*Dir)(nil))
var _ = (fs.NodeMkdirer)((*Dir)(nil))
var _ = (fs.NodeUnlinker)((*Dir)(nil))
var _ = (fs.NodeRmdirer)((*Dir)(nil))
var _ = (fs.NodeRenamer)((*Dir)(nil))
var _ = (fs.NodeSetattrer)((*Dir)(nil))

func (r *Dir) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	//log.Printf("(*Dir).Rename(%s (%s -> %s) (flags: %#x))", r.path, name, newName, flags)
	newD, ok := newParent.(*Dir)
	if !ok {
		//log.Printf("Cannot rename to non-directory parent.")
		return syscall.EINVAL
	}
	if r != newD {
		//log.Printf("Cannot move from one place to another. (%s -> %s)", r.path, newD.path)
		return syscall.EINVAL
	}
	stat := proto.Stat{
		Type:   math.MaxUint16,
		Dev:    math.MaxUint32,
		Qid:    proto.Qid{Qtype: math.MaxUint8, Vers: math.MaxUint32, Uid: math.MaxUint64},
		Mode:   math.MaxUint32,
		Atime:  math.MaxUint32,
		Mtime:  math.MaxUint32,
		Length: math.MaxUint64,
		Name:   newName,
		Uid:    "",
		Gid:    "",
		Muid:   "",
	}
	err := r.client.WStat(path.Join(r.path, name), &stat)
	if err != nil {
		log.Printf("WSTAT RETURNED ERROR: %s\n", err)
		return syscall.ENOENT
	}
	r.dirTTL = time.Time{}
	r.statTTL = time.Time{}
	return 0
}

func (r *Dir) Unlink(ctx context.Context, name string) syscall.Errno {
	//log.Printf("(*Dir).Unlink(%s)", r.path)
	err := r.client.Remove(path.Join(r.path, name))
	if err != nil {
		//log.Printf("Unlink failed: %s\n", err)
		return syscall.EINVAL
	}
	r.dirTTL = time.Time{}
	r.statTTL = time.Time{}
	return 0
}

func (r *Dir) Rmdir(ctx context.Context, name string) syscall.Errno {
	//log.Printf("(*Dir).Rmdir(%s)", r.path)
	err := r.client.Remove(path.Join(r.path, name))
	if err != nil {
		//log.Printf("Unlink failed: %s\n", err)
		return syscall.EINVAL
	}
	r.dirTTL = time.Time{}
	r.statTTL = time.Time{}
	return 0
}

func (r *Dir) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	//log.Printf("(*Dir).Mkdir(%s)", r.path)
	fullPath := path.Join(r.path, name)
	//log.Printf("Mkdir(%s)", fullPath)
	file, err := r.client.Create(fullPath, os.FileMode(mode|proto.DMDIR))
	if err != nil {
		if err.Error() == "Permission denied." {
			return nil, syscall.EACCES
		}
		//log.Printf("Error creating [%s]: %s", r.path, err)
		return nil, syscall.EINVAL
	}
	defer file.Close()
	r.dirTTL = time.Time{}
	r.statTTL = time.Time{}
	r.dirCache = append(r.dirCache, proto.Stat{
		Type:   0,
		Dev:    0,
		Qid:    proto.Qid{Qtype: math.MaxUint8, Vers: math.MaxUint32, Uid: math.MaxUint64},
		Mode:   mode | proto.DMDIR,
		Atime:  0,
		Mtime:  0,
		Length: 0,
		Name:   name,
		Uid:    "",
		Gid:    "",
		Muid:   "",
	})
	dir := &Dir{client: r.client, path: fullPath}
	dirPut(fullPath, dir)
	return r.NewInode(ctx, dir, fs.StableAttr{Mode: fuse.S_IFDIR, Ino: crc64.Checksum([]byte(fullPath), crc64Table)}), 0
}

func (r *Dir) oldGetattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	//log.Printf("(*Dir).oldGetattr(%s)", r.path)
	if r.statCache == nil || time.Now().After(r.statTTL) {
		//log.Printf("oldGetattr(%s)", r.path)
		stat, err := r.client.Stat(r.path)
		if err != nil {
			log.Printf("STAT RETURNED ERROR: %s\n", err)
			return syscall.ENOENT
		}
		r.statCache = stat
		r.statTTL = time.Now().Add(CacheTTL)
	}
	out.AttrValid = ncTTL
	out.Nlink = 1
	out.Ino = r.statCache.Qid.Uid
	out.Mode = r.statCache.Mode
	out.Size = r.statCache.Length
	out.Mtime = uint64(r.statCache.Mtime)
	out.Uid = uidForUser(r.statCache.Uid)
	out.Gid = gidForGroup(r.statCache.Gid)
	return 0
}

func (r *Dir) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	//log.Printf("(*Dir).Getattr(%s)", r.path)
	if dir := dirGet(path.Dir(r.path)); dir != nil {
		_, errno := dir.Readdir(ctx)
		if errno > 0 {
			return errno
		}
		base := path.Base(r.path)
		for _, stat := range dir.dirCache {
			if stat.Name == base {
				out.AttrValid = ncTTL
				out.Nlink = 1
				out.Ino = stat.Qid.Uid
				out.Mode = stat.Mode
				out.Size = stat.Length
				out.Mtime = uint64(stat.Mtime)
				return 0
			}
		}
	}
	return r.oldGetattr(ctx, f, out)
}

func (r *Dir) Setattr(ctx context.Context, h fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	//log.Printf("(*Dir).SetAttr(%s)", r.path)
	stat := proto.Stat{
		Type:   math.MaxUint16,
		Dev:    math.MaxUint32,
		Qid:    proto.Qid{Qtype: math.MaxUint8, Vers: math.MaxUint32, Uid: math.MaxUint64},
		Mode:   math.MaxUint32,
		Atime:  math.MaxUint32,
		Mtime:  math.MaxUint32,
		Length: math.MaxUint64,
		Name:   "",
		Uid:    "",
		Gid:    "",
		Muid:   "",
	}
	send := false
	if newMode, ok := in.GetMode(); ok {
		stat.Mode = newMode
		send = true
	}
	if newSize, ok := in.GetSize(); ok {
		stat.Length = newSize
		send = true
	}
	if send {
		err := r.client.WStat(r.path, &stat)
		if err != nil {
			log.Printf("WSTAT RETURNED ERROR: %s\n", err)
			return syscall.ENOENT
		}
	}
	out.Mode = stat.Mode
	out.Size = stat.Length
	out.Mtime = uint64(stat.Mtime)
	if dir := dirGet(path.Dir(r.path)); dir != nil {
		dir.dirTTL = time.Time{}
		dir.statTTL = time.Time{}
	}
	out.Uid = uidForUser(stat.Uid)
	out.Gid = gidForGroup(stat.Gid)
	return 0
}

func (r *Dir) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (node *fs.Inode, fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	//log.Printf("(*Dir).Create(%s)", path.Join(r.path, name))
	file, err := r.client.Create(path.Join(r.path, name), os.FileMode(mode))
	if err != nil {
		if err.Error() == "Permission denied." {
			return nil, nil, 0, syscall.EACCES
		}
		//log.Printf("Error creating [%s]: %s", r.path, err)
		return nil, nil, 0, syscall.EINVAL
	}
	r.dirTTL = time.Time{}
	r.statTTL = time.Time{}
	r.dirCache = append(r.dirCache, proto.Stat{
		Type:   0,
		Dev:    0,
		Qid:    proto.Qid{Qtype: math.MaxUint8, Vers: math.MaxUint32, Uid: math.MaxUint64},
		Mode:   mode,
		Atime:  0,
		Mtime:  0,
		Length: 0,
		Name:   name,
		Uid:    "",
		Gid:    "",
		Muid:   "",
	})
	fullPath := path.Join(r.path, name)
	fileNode := &FileNode{client: r.client, path: fullPath}
	return r.NewInode(ctx, fileNode, fs.StableAttr{Ino: crc64.Checksum([]byte(fullPath), crc64Table)}), &File{file, fileNode}, fuse.FOPEN_DIRECT_IO, 0
}

func (r *Dir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	//log.Printf("(*Dir).Lookup(%s): %s", r.path, name)
	if r.dirCache == nil || time.Now().After(r.dirTTL) {
		//log.Printf("LOOKUP READDIR(%s)\n", r.path)
		stats, err := r.client.Readdir(r.path)
		if err != nil {
			return nil, syscall.EPIPE
		}
		r.dirCache = stats
		r.dirTTL = time.Now().Add(CacheTTL)
	}
	for _, stat := range r.dirCache {
		if stat.Name == name {
			out.EntryValid = ncTTL
			out.AttrValid = ncTTL
			out.Nlink = 1
			out.Ino = stat.Qid.Uid
			out.Mode = stat.Mode
			out.Size = stat.Length
			out.Mtime = uint64(stat.Mtime)
			out.Uid = uidForUser(stat.Uid)
			out.Gid = gidForGroup(stat.Gid)
			fullPath := path.Join(r.path, name)
			if stat.Mode&proto.DMDIR > 0 {
				if dir := dirGet(fullPath); dir != nil {
					return r.NewInode(ctx, dir, fs.StableAttr{Mode: fuse.S_IFDIR, Ino: crc64.Checksum([]byte(fullPath), crc64Table)}), 0
				}
				dir := &Dir{client: r.client, path: fullPath}
				dirPut(fullPath, dir)
				return r.NewInode(ctx, dir, fs.StableAttr{Mode: fuse.S_IFDIR, Ino: crc64.Checksum([]byte(fullPath), crc64Table)}), 0
			}
			return r.NewInode(ctx, &FileNode{client: r.client, path: fullPath}, fs.StableAttr{Ino: crc64.Checksum([]byte(fullPath), crc64Table)}), 0
		}
	}
	return nil, syscall.ENOENT
}

func (r *Dir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	//log.Printf("(*Dir).Readdir(%s)", r.path)
	if r.dirCache == nil || time.Now().After(r.dirTTL) {
		//log.Printf("ACTUAL READDIR(%s)\n", r.path)
		stats, err := r.client.Readdir(r.path)
		if err != nil {
			return nil, syscall.EPIPE
		}
		r.dirCache = stats
		r.dirTTL = time.Now().Add(CacheTTL)
	}
	entries := make([]fuse.DirEntry, 0)
	for _, stat := range r.dirCache {
		var mode uint32 = 0
		if stat.Mode&proto.DMDIR > 0 {
			mode = fuse.S_IFDIR
		}
		entries = append(entries, fuse.DirEntry{Name: stat.Name, Mode: mode})
	}

	return fs.NewListDirStream(entries), 0
}

type FileNode struct {
	fs.Inode
	client *client.Client
	path   string
}

type File struct {
	file *client.File
	node *FileNode
}

var _ = (fs.NodeOpener)((*FileNode)(nil))
var _ = (fs.NodeGetattrer)((*FileNode)(nil))
var _ = (fs.NodeSetattrer)((*FileNode)(nil))
var _ = (fs.NodeFsyncer)((*FileNode)(nil))
var _ = (fs.FileReader)((*File)(nil))
var _ = (fs.FileWriter)((*File)(nil))
var _ = (fs.FileFlusher)((*File)(nil))
var _ = (fs.FileReleaser)((*File)(nil))
var _ = (fs.FileSetattrer)((*File)(nil))

func convertFlag(mode uint32) proto.Mode {
	var m proto.Mode
	switch int(mode & 0x0F) {
	case os.O_RDONLY:
		m = proto.Oread
	case os.O_WRONLY:
		m = proto.Owrite
	case os.O_RDWR:
		m = proto.Ordwr
	}
	if (int(mode) & os.O_TRUNC) > 0 {
		m |= proto.Otrunc
	}
	return m
}

func (f *FileNode) Fsync(ctx context.Context, fh fs.FileHandle, flags uint32) syscall.Errno {
	//log.Printf("FUSE: Fsync(%s)\n", f.path)
	return 0
}

func (f *FileNode) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	//log.Printf("(*FileNode).Open(%s, %#x -> %#x)\n", f.path, flags, convertFlag(flags))
	file, err := f.client.Open(f.path, convertFlag(flags))
	if err != nil {
		if err.Error() == "Permission denied." {
			return nil, 0, syscall.EACCES
		}
		//log.Printf("FUSE: Open(%s) -> Error: %s", f.path, err)
		return nil, 0, syscall.EINVAL
	}
	if *dio {

		return &File{file, f}, fuse.FOPEN_DIRECT_IO, 0
	}
	// TODO: Optimize
	stat, err := f.client.Stat(f.path)
	if err != nil {
		log.Printf("STAT RETURNED ERROR: %s\n", err)
		return nil, 0, syscall.ENOENT
	}
	if stat.Length == 0 {
		return &File{file, f}, fuse.FOPEN_DIRECT_IO, 0
	}

	return &File{file, f}, 0, 0
	//log.Printf("FUSE: Open(%s) -> OK\n", f.path)
	//return &File{file, f}, fuse.FOPEN_DIRECT_IO, 0
	//Inode.NotifyContent
	//return &File{file, f}, fuse.FOPEN_KEEP_CACHE, 0
}

func (f *FileNode) oldGetattr(ctx context.Context, h fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	//log.Printf("FileNode.oldGetattr(%s)", f.path)
	stat, err := f.client.Stat(f.path)
	if err != nil {
		log.Printf("STAT RETURNED ERROR: %s\n", err)
		return syscall.ENOENT
	}
	out.AttrValid = ncTTL
	out.Nlink = 1
	out.Ino = stat.Qid.Uid
	out.Mode = stat.Mode
	out.Size = stat.Length
	out.Mtime = uint64(stat.Mtime)
	out.Uid = uidForUser(stat.Uid)
	out.Gid = gidForGroup(stat.Gid)
	return 0
}

func (f *FileNode) Getattr(ctx context.Context, h fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	//log.Printf("(*FileNode).Getattr(%s)", f.path)
	if dir := dirGet(path.Dir(f.path)); dir != nil {
		_, errno := dir.Readdir(ctx)
		if errno > 0 {
			return errno
		}
		base := path.Base(f.path)
		for _, stat := range dir.dirCache {
			if stat.Name == base {
				out.AttrValid = ncTTL
				out.Nlink = 1
				out.Ino = stat.Qid.Uid
				out.Mode = stat.Mode
				out.Size = stat.Length
				out.Mtime = uint64(stat.Mtime)
				out.Uid = uidForUser(stat.Uid)
				out.Gid = gidForGroup(stat.Gid)
				return 0
			}
		}
	}
	return f.oldGetattr(ctx, h, out)
}

func (f *FileNode) Setattr(ctx context.Context, h fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	//log.Printf("(*FileNode).SetAttr(%s)", f.path)
	stat := proto.Stat{
		Type:   math.MaxUint16,
		Dev:    math.MaxUint32,
		Qid:    proto.Qid{Qtype: math.MaxUint8, Vers: math.MaxUint32, Uid: math.MaxUint64},
		Mode:   math.MaxUint32,
		Atime:  math.MaxUint32,
		Mtime:  math.MaxUint32,
		Length: math.MaxUint64,
		Name:   "",
		Uid:    "",
		Gid:    "",
		Muid:   "",
	}
	send := false
	if newMode, ok := in.GetMode(); ok {
		stat.Mode = newMode
		send = true
	}
	if newSize, ok := in.GetSize(); ok {
		stat.Length = newSize
		send = true
	}
	if send {
		//log.Printf("SENDING WSTAT")
		err := f.client.WStat(f.path, &stat)
		if err != nil {
			log.Printf("WSTAT RETURNED ERROR: %s\n", err)
			return syscall.ENOENT
		}
	}
	out.Mode = stat.Mode
	out.Size = stat.Length
	out.Mtime = uint64(stat.Mtime)
	if dir := dirGet(path.Dir(f.path)); dir != nil {
		dir.dirTTL = time.Time{}
		dir.statTTL = time.Time{}
	}
	out.Uid = uidForUser(stat.Uid)
	out.Gid = gidForGroup(stat.Gid)
	return 0
}

func (f *File) Flush(ctx context.Context) syscall.Errno {
	//log.Printf("(*File).Flush(%s)\n", f.node.path)
	return 0
}

func (f *File) Release(ctx context.Context) syscall.Errno {
	//log.Printf("(*File).Release(%s)\n", f.node.path)
	err := f.file.Close()
	if err != nil {
		return syscall.EINVAL
	}
	return 0
}

func (f *File) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	//log.Printf("(*File).Read(%s, off: %d, len: %d)", f.node.path, off, len(dest))
	n, err := f.file.ReadAt(dest, off)
	if err != nil {
		if err == io.EOF {
			return fuse.ReadResultData(dest[:n]), 0
		}
		log.Printf("Error reading file: %s", err)
		return nil, syscall.EINVAL
	}
	return fuse.ReadResultData(dest[:n]), 0
}

func (f *File) Setattr(ctx context.Context, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	//log.Printf("(*File).SetAttr(%s)", f.node.path)
	stat, err := f.node.client.Stat(f.node.path)
	if err != nil {
		log.Printf("STAT RETURNED ERROR: %s\n", err)
		return syscall.ENOENT
	}
	stat.Mode = in.Mode
	stat.Length = in.Size
	err = f.node.client.WStat(f.node.path, stat)
	if err != nil {
		log.Printf("WSTAT RETURNED ERROR: %s\n", err)
		return syscall.ENOENT
	}
	out.Mode = stat.Mode
	out.Size = stat.Length
	out.Mtime = uint64(stat.Mtime)
	if dir := dirGet(path.Dir(f.node.path)); dir != nil {
		dir.dirTTL = time.Time{}
		dir.statTTL = time.Time{}
	}
	out.Uid = uidForUser(stat.Uid)
	out.Gid = gidForGroup(stat.Gid)
	return 0
}

func (f *File) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	//log.Printf("f.Write(data(%d), off: %d)", len(data), off)
	n, err := f.file.WriteAt(data, off)
	if err != nil {
		//log.Printf("Error writing file: %s", err)
		return uint32(n), syscall.EINVAL
	}
	if dir := dirGet(path.Dir(f.node.path)); dir != nil {
		dir.dirTTL = time.Time{}
		dir.statTTL = time.Time{}
	}
	return uint32(n), 0
}

type ReadWriteCloser struct {
	io.ReadCloser
	io.WriteCloser
}

func (r *ReadWriteCloser) Close() error {
	err1 := r.ReadCloser.Close()
	err2 := r.WriteCloser.Close()
	if err1 != nil && err2 != nil {
		return fmt.Errorf("Read and Write failed to close: [Read: %s], [Write: %s]", err1, err2)
	} else if err1 != nil {
		return fmt.Errorf("Read failed to close: %s", err1)
	} else if err2 != nil {
		return fmt.Errorf("Write failed to close: %s", err2)
	}
	return nil
}

var dio *bool = flag.Bool("dio", false, "Force the use of Direct IO - bypasses caching, read-ahead. Fixes 9p files with wrong reported lengths.")

func main() {
	var defaultUser string
	u, err := user.Current()
	if err != nil {
		defaultUser = "none"
		log.Fatal("Failed to determine current user: %v", err)
	} else {
		defaultUser = u.Username
		su, _ := strconv.Atoi(u.Uid)
		sg, _ := strconv.Atoi(u.Gid)
		sysUser = uint32(su)
		sysGroup = uint32(sg)
	}

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "  %s [options] address mountpoint\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "  %s [options] -srv local_service mountpoint\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "  %s [options] -s mountpoint\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	debug := flag.Bool("debug", false, "Prints FUSE debugging information.")
	verbose := flag.Bool("v", false, "Makes the 9p protocol verbose, printing all incoming and outgoing messages.")
	username := flag.String("user", defaultUser, "User to log in as")
	aname := flag.String("aname", "", "Specific file system to attach to, if any")
	auth := flag.Bool("a", false, "Enable plan9 auth")
	stdio := flag.Bool("s", false, "Speak 9p over standard input/output")
	srv := flag.Bool("srv", false, "Attach to a 9p service, not an address")
	other := flag.Bool("other", false, "Enable the allow_other mount flag (See: mount.fuse(8))")
	usetls := flag.Bool("tls", false, "Use TLS to encrypt communication with the server.")
	certfile := flag.String("certfile", "", "If provided, use the certificate to authenticate to the server. Implies -tls")
	cachetime := flag.String("cachetime", "10s", "If provided, this is the amount of time various things (directory contents, file stats, etc) are cached before being recalculated. Must be in the format that time.ParseDuration accepts.")

	flag.Parse()

	unknownUID = unusedUid()
	unknownGID = unusedGid()
	authUser = *username
	go9p.Verbose = *verbose

	t, err := time.ParseDuration(*cachetime)
	if err != nil {
		log.Fatalf("Failed to parse cache time: %v\n", err)
	}
	CacheTTL = t
	ncTTL = uint64(t / time.Second)

	var clientOpts []client.Option
	if *auth {
		clientOpts = append(clientOpts, client.WithAuth(client.Plan9Auth))
	}

	//var network, addr string
	var c *client.Client
	var mountpoint string
	if *stdio {
		if len(flag.Args()) < 1 {
			flag.Usage()
			os.Exit(1)
		}
		s := &ReadWriteCloser{os.Stdin, os.Stdout}
		log.Printf("Mapping authenticated user %s to system user %s", authUser, u.Username)
		c, err = client.NewClient(s, *username, *aname, clientOpts...)
		if err != nil {
			log.Fatal(err)
		}
		mountpoint = flag.Arg(0)
	} else {
		if len(flag.Args()) < 2 {
			flag.Usage()
			os.Exit(1)
		}
		network := "tcp"
		addr := flag.Arg(0)
		if _, err := os.Stat(addr); err == nil {
			// Probably a unix socket.
			network = "unix"
		}
		if *srv {
			network = "unix"
			ns := fans.Namespace()
			addr = path.Join(ns, addr)
		}

		mountpoint = flag.Arg(1)
		if *usetls {
			var crt *tls.Certificate
			var ca *x509.Certificate
			if *certfile != "" {
				ecrt, eca, err := cert.LoadTLSCert(*certfile)
				if err != nil {
					log.Fatalf("Failed to load certificate: %s", err)
				}
				u, err := cert.TLSCertUser(&ecrt)
				if err != nil {
					log.Fatalf("Failed to find certificate user: %s", err)
				}
				log.Printf("TLS USER IS: %v\n", u)
				authUser = u
				crt = &ecrt
				ca = eca
			}
			log.Printf("Mapping authenticated user %s to system user %s", authUser, u.Username)
			c, err = client.DialTLS(network, addr, *username, *aname, crt, ca)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			log.Printf("Mapping authenticated user %s to system user %s", authUser, u.Username)
			c, err = client.Dial(network, addr, *username, *aname, clientOpts...)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	opts := &fs.Options{
		UID: uint32(os.Geteuid()),
		GID: uint32(os.Getgid()),
		MountOptions: fuse.MountOptions{
			DirectMount: true,
			AllowOther:  *other,
		},
	}
	opts.Debug = *debug
	root := &StatDir{Dir{client: c, path: "/"}, 0777}
	//dirPut("/", root)
	server, err := fs.Mount(mountpoint, root, opts)
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}
	server.Wait()
}
