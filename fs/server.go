package fs

import (
	"fmt"
	"log"
	"math"
	"sync"

	"github.com/knusbaum/go9p"
	"github.com/knusbaum/go9p/proto"

	"github.com/Plan9-Archive/libauth"
)

type fidInfo struct {
	n          FSNode
	openMode   proto.Mode
	openOffset uint64
	extra      interface{}
}

func newFidInfo(n FSNode) *fidInfo {
	return &fidInfo{
		n:        n,
		openMode: proto.None,
	}
}

type conn struct {
	uname  string
	connID uint32
	//fids  map[uint32]*fidInfo
	fids sync.Map
}

func (c *conn) Uname() string {
	return c.uname
}

func (c *conn) toConnFid(fid uint32) uint64 {
	bcid := uint64(c.connID)
	bcid = bcid << 32
	bcid = bcid | uint64(fid)
	return bcid
}

type server struct {
	fs         *FS
	currConnId uint32
}

// Server returns a go9p.Srv instance which will
// serve the 9p2000 protocol.
func (fs *FS) Server() go9p.Srv {
	return &server{fs: fs}
}

func (s *server) NewConn() go9p.Conn {
	s.currConnId += 1
	return &conn{uname: "none", connID: s.currConnId}
}

func (_ *server) Version(gc go9p.Conn, t *proto.TRVersion) (proto.FCall, error) {
	var reply proto.TRVersion
	if t.Type == proto.Tversion {
		reply = *t
		reply.Type = proto.Rversion
		return &reply, nil
	} else {
		return nil, fmt.Errorf("Cannot reply to type %d\n", t.Type)
	}
}

func (s *server) Auth(gc go9p.Conn, t *proto.TAuth) (proto.FCall, error) {
	if !s.fs.doAuth {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Authentication Not Supported."}, nil
	}
	c := gc.(*conn)

	stream := NewBlockingStream(10, true)
	authFile := NewStreamFile(
		s.fs.NewStat("auth", "glenda", "glenda", 0666),
		stream,
	)

	err := authFile.Open(c.toConnFid(t.Afid), proto.Ordwr)
	if err != nil {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, err.Error()}, nil
	}
	info := &fidInfo{
		n:        authFile,
		openMode: proto.Ordwr,
	}
	c.fids.Store(t.Afid, info)

	go func(s BiDiStream) {
		defer s.Close()
		ai, err := libauth.Proxy(s, "proto=p9any role=server user=%s", t.Uname)
		if err != nil {
			info.extra = err
			log.Printf("Authentication Error: %s", err)
		} else {
			info.extra = ai
			log.Printf("AuthInfo: [Cuid: %s, Suid: %s, Cap: %s]", ai.Cuid, ai.Suid, ai.Cap)
		}

	}(stream)

	return &proto.RAuth{proto.Header{proto.Rauth, t.Tag}, authFile.Stat().Qid}, nil
}

func (s *server) Attach(gc go9p.Conn, t *proto.TAttach) (proto.FCall, error) {
	c := gc.(*conn)

	if !s.fs.doAuth {
		c.uname = t.Uname
		c.fids.Store(t.Fid, newFidInfo(s.fs.Root))
		return &proto.RAttach{proto.Header{proto.Rattach, t.Tag}, s.fs.Root.Stat().Qid}, nil
	}

	i, ok := c.fids.Load(t.Afid)
	if !ok {
		return &proto.RError{proto.Header{t.Type, t.Tag}, "Bad Afid."}, nil
	}
	info := i.(*fidInfo)
	if err, ok := info.extra.(error); ok {
		return &proto.RError{proto.Header{t.Type, t.Tag}, err.Error()}, nil
	}

	ai := info.extra.(*libauth.AuthInfo)
	//	if t.Uname != ai.Cuid {
	//		return &proto.RError{proto.Header{t.Type, t.Tag}, "Bad attach uname"}, nil
	//	}
	c.uname = ai.Cuid
	c.fids.Store(t.Fid, newFidInfo(s.fs.Root))
	return &proto.RAttach{proto.Header{proto.Rattach, t.Tag}, s.fs.Root.Stat().Qid}, nil
}

func (_ *server) Flush(gc go9p.Conn, t *proto.TFlush) (proto.FCall, error) {
	return &proto.RFlush{proto.Header{proto.Rflush, t.Tag}}, nil
}

func (s *server) Walk(gc go9p.Conn, t *proto.TWalk) (proto.FCall, error) {
	c := gc.(*conn)
	i, ok := c.fids.Load(t.Fid)
	if !ok {
		return &proto.RError{proto.Header{t.Type, t.Tag}, "Bad Fid."}, nil
	}
	info := i.(*fidInfo)
	file := info.n
	if t.Nwname > 0 && t.Wname[0] == ".." {
		parent := file.Parent()
		if parent != nil {
			//c.fids[t.Newfid] = newFidInfo(parent)
			c.fids.Store(t.Newfid, newFidInfo(parent))
			qids := make([]proto.Qid, 1)
			qids[0] = parent.Stat().Qid
			return &proto.RWalk{proto.Header{proto.Rwalk, t.Tag}, 1, qids}, nil
		} else {
			return &proto.RWalk{proto.Header{proto.Rwalk, t.Tag}, 0, nil}, nil
		}
	}

	qids := make([]proto.Qid, 0)
	for i := 0; i < int(t.Nwname); i++ {
		if dir, ok := file.(Dir); ok {
			file, ok = dir.Children()[t.Wname[i]]
			if !ok {
				if s.fs.WalkFail == nil {
					return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "No such path"}, nil
				}
				f, err := s.fs.WalkFail(s.fs, dir, t.Wname[i])
				if err != nil {
					return &proto.RError{proto.Header{proto.Rerror, t.Tag}, err.Error()}, nil
				}
				if f == nil {
					return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "No such path"}, nil
				}
				dir.AddChild(f)
				file = f
			}
			qids = append(qids, file.Stat().Qid)
		} else {
			return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "No such path"}, nil
		}
	}
	//c.fids[t.Newfid] = newFidInfo(file)
	c.fids.Store(t.Newfid, newFidInfo(file))
	return &proto.RWalk{proto.Header{proto.Rwalk, t.Tag}, uint16(len(qids)), qids}, nil
}

func (_ *server) Open(gc go9p.Conn, t *proto.TOpen) (proto.FCall, error) {
	c := gc.(*conn)
	//info, ok := c.fids[t.Fid]
	i, ok := c.fids.Load(t.Fid)
	if !ok {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Bad Fid."}, nil
	}
	info := i.(*fidInfo)
	if info.openMode != proto.None {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Fid already open."}, nil
	}
	if !openPermission(info.n, c.uname, t.Mode&0x0F) {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Permission denied."}, nil
	}

	switch n := info.n.(type) {
	case Dir:
		if (t.Mode&0x0F) == proto.Owrite ||
			(t.Mode&0x0F) == proto.Ordwr {
			return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Cannot write to directory."}, nil
		}
		children := n.Children()
		cl := make([]FSNode, 0)
		for _, c := range children {
			cl = append(cl, c)
		}
		info.extra = cl
	case File:
		err := n.Open(c.toConnFid(t.Fid), t.Mode)
		if err != nil {
			return &proto.RError{proto.Header{proto.Rerror, t.Tag}, err.Error()}, nil
		}
	}
	info.openMode = t.Mode
	info.openOffset = info.n.Stat().Length

	return &proto.ROpen{proto.Header{proto.Ropen, t.Tag}, info.n.Stat().Qid, proto.IOUnit}, nil
}

func (s *server) Create(gc go9p.Conn, t *proto.TCreate) (proto.FCall, error) {
	c := gc.(*conn)
	//info, ok := c.fids[t.Fid]
	i, ok := c.fids.Load(t.Fid)
	if !ok {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Bad Fid."}, nil
	}
	info := i.(*fidInfo)
	if !openPermission(info.n, c.uname, proto.Owrite) {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Permission denied."}, nil
	}

	if dir, ok := info.n.(Dir); ok {
		var new FSNode
		var err error
		if t.Perm&proto.DMDIR != 0 {
			if s.fs.CreateDir != nil {
				new, err = s.fs.CreateDir(s.fs, dir, c.uname, t.Name, t.Perm, t.Mode)
			} else {
				err = fmt.Errorf("Cannot create directories.")
			}
		} else {
			if s.fs.CreateFile != nil {
				new, err = s.fs.CreateFile(s.fs, dir, c.uname, t.Name, t.Perm, t.Mode)
			} else {
				err = fmt.Errorf("Cannot create files.")
			}
		}
		if err != nil {
			return &proto.RError{proto.Header{proto.Rerror, t.Tag}, err.Error()}, nil
		}
		info = newFidInfo(new)
		info.openMode = proto.Mode(t.Mode)
		info.openOffset = 0
		//c.fids[t.Fid] = info
		c.fids.Store(t.Fid, info)
		if f, ok := new.(File); ok {
			err := f.Open(c.toConnFid(t.Fid), proto.Mode(t.Mode))
			if err != nil {
				return &proto.RError{proto.Header{proto.Rerror, t.Tag}, err.Error()}, nil
			}
		}
		return &proto.RCreate{proto.Header{proto.Rcreate, t.Tag}, new.Stat().Qid, proto.IOUnit}, nil
	} else if f, ok := info.n.(File); ok {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, f.Stat().Name + ": IS A FILE Not a directory"}, nil
	} else {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, info.n.Stat().Name + ": Not a directory"}, nil
	}
}

func (_ *server) Read(gc go9p.Conn, t *proto.TRead) (proto.FCall, error) {
	c := gc.(*conn)
	if t.Count > proto.IOUnit {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Read size too large."}, nil
	}
	//info, ok := c.fids[t.Fid]
	i, ok := c.fids.Load(t.Fid)
	if !ok {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Bad Fid."}, nil
	}
	info := i.(*fidInfo)

	openmode := info.openMode & 0x0F
	// TODO: Can't we just check against None?
	if openmode != proto.Oread &&
		openmode != proto.Ordwr &&
		openmode != proto.Oexec {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "1File not opened."}, nil
	}

	switch n := info.n.(type) {
	case File:
		data, err := n.Read(c.toConnFid(t.Fid), t.Offset, uint64(t.Count))
		if err != nil {
			return &proto.RError{proto.Header{proto.Rerror, t.Tag}, err.Error()}, nil
		}
		return &proto.RRead{proto.Header{proto.Rread, t.Tag}, uint32(len(data)), data}, nil
	case Dir:
		return readDir(t, info), nil
	}
	return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "2File not opened."}, nil
}

func readDir(t *proto.TRead, info *fidInfo) proto.FCall {
	contents := make([]byte, 0)
	children := info.extra.([]FSNode)

	var length uint64

	// determine which child to start with based on read offset.
	startIndex := -1
	for i, c := range children {
		st := c.Stat()
		nextLength := uint64(st.ComposeLength())
		if length+nextLength > t.Offset {
			startIndex = i
			break
		}
		length = length + nextLength
	}

	// Offset is beyond the end of our list.
	if startIndex < 0 {
		return &proto.RRead{proto.Header{proto.Rread, t.Tag}, 0, nil}
	}

	for _, f := range children[startIndex:] {
		st := f.Stat()
		nextLength := uint32(st.ComposeLength())
		if uint32(len(contents))+nextLength > t.Count {
			break
		}
		contents = append(contents, st.Compose()...)
	}
	return &proto.RRead{proto.Header{proto.Rread, t.Tag}, uint32(len(contents)), contents}
}

func (_ *server) Write(gc go9p.Conn, t *proto.TWrite) (proto.FCall, error) {
	c := gc.(*conn)
	//info, ok := c.fids[t.Fid]
	i, ok := c.fids.Load(t.Fid)
	if !ok {
		// TODO: Handle Auth
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Bad Fid."}, nil
	}
	info := i.(*fidInfo)

	if (info.openMode&0x0F) != proto.Owrite &&
		(info.openMode&0x0F) != proto.Ordwr {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "File not opened for write."}, nil
	} else if (info.n.Stat().Mode & proto.DMDIR) != 0 {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Cannot write to directory."}, nil
	}

	offset := t.Offset
	if info.openMode&0x10 == 0 {
		// If we're not truncating, 0 offset is from EOF.
		offset += info.openOffset
	}

	if f, ok := info.n.(File); ok {
		n, err := f.Write(c.toConnFid(t.Fid), offset, t.Data)
		if err != nil {
			return &proto.RError{proto.Header{proto.Rerror, t.Tag}, err.Error()}, nil
		}
		return &proto.RWrite{proto.Header{proto.Rwrite, t.Tag}, n}, nil
	} else {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Cannot write to directory."}, nil
	}
}

func (_ *server) Clunk(gc go9p.Conn, t *proto.TClunk) (proto.FCall, error) {
	c := gc.(*conn)
	//info, ok := c.fids[t.Fid]
	//delete(c.fids, t.Fid)
	i, ok := c.fids.Load(t.Fid)
	c.fids.Delete(t.Fid)
	if !ok {
		return &proto.RClunk{proto.Header{proto.Rclunk, t.Tag}}, nil
	}
	info := i.(*fidInfo)

	if info.openMode != proto.None {
		if f, ok := info.n.(File); ok {
			err := f.Close(c.toConnFid(t.Fid))
			if err != nil {
				return &proto.RError{proto.Header{proto.Rerror, t.Tag}, err.Error()}, nil
			}
		}
	}
	return &proto.RClunk{proto.Header{proto.Rclunk, t.Tag}}, nil
}

func (s *server) Remove(gc go9p.Conn, t *proto.TRemove) (proto.FCall, error) {
	c := gc.(*conn)
	i, ok := c.fids.Load(t.Fid)
	if !ok {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Bad Fid."}, nil
	}
	info := i.(*fidInfo)

	if !openPermission(info.n, c.uname, proto.Owrite) {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Permission denied."}, nil
	}

	var err error
	if s.fs.RemoveFile != nil {
		err = s.fs.RemoveFile(s.fs, info.n)
	} else {
		err = fmt.Errorf("Cannot delete files.")
	}
	if err != nil {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, err.Error()}, nil
	}
	return &proto.RRemove{proto.Header{proto.Rremove, t.Tag}}, nil
}

func (_ *server) Stat(gc go9p.Conn, t *proto.TStat) (proto.FCall, error) {
	c := gc.(*conn)
	i, ok := c.fids.Load(t.Fid)
	if !ok {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Bad Fid."}, nil
	}
	info := i.(*fidInfo)

	return &proto.RStat{proto.Header{proto.Rstat, t.Tag}, info.n.Stat()}, nil
}

/* The name can be changed by anyone with write permission in
 * the parent directory; it is an error to change the name to
 * that of an existing file.
 *
 * The length can be changed (affecting the actual length of
 * the file) by anyone with write permission on the file. It
 * is an error to attempt to set the length of a directory to
 * a non-zero value, and servers may decide to reject length
 * changes for other reasons.
 *
 * The mode and mtime can be changed by the owner of the file
 * or the group leader of the file's current group.
 *
 * The directory bit cannot be changed by a wstat;
 *
 * the other defined permission and mode bits can.
 *
 * The gid can be changed: by the owner if also a member of
 * the new group; or by the group leader of the file's current
 * group if also leader of the new group (see intro(5) for
 * more information about permissions and users(6) for users
 * and groups)
 */

/* A wstat request can avoid modifying some properties of the
 * file by providing explicit ``don't touch'' values in the
 * stat data that is sent: zero-length strings for text
 * values and the maximum unsigned value of appropriate size
 * for inte- gral values. As a special case, if all the
 * elements of the directory entry in a Twstat message are
 * ``don't touch'' val- ues, the server may interpret it as a
 * request to guarantee that the contents of the associated
 * file are committed to stable storage before the Rwstat
 * message is returned. (Con- sider the message to mean,
 * ``make the state of the file exactly what it claims to be.'')
 */
func (_ *server) Wstat(gc go9p.Conn, t *proto.TWstat) (proto.FCall, error) {
	c := gc.(*conn)
	i, ok := c.fids.Load(t.Fid)
	if !ok {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Bad Fid."}, nil
	}
	info := i.(*fidInfo)

	stat := info.n.Stat()
	newstat := &t.Stat
	relation := userRelation(c.uname, info.n)

	{
		// Need to check all this stuff before we change *ANYTHING*
		// The server needs to accept ALL the changes or none of them.
		if len(newstat.Name) != 0 {
			if relation != ugo_user {
				fmt.Println("Can't change name. Not owner.")
				return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Permission denied."}, nil
			}
		}

		if newstat.Length != math.MaxUint64 {
			if !openPermission(info.n, c.uname, proto.Owrite) {
				fmt.Println("Can't alter length. Don't have write permission.")
				return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Permission denied."}, nil
			}
		}

		if newstat.Mode != math.MaxUint32 {
			if relation != ugo_user {
				fmt.Println("Can't alter mode. Not owner.")
				return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Permission denied."}, nil
			}
		}

		if newstat.Mtime != math.MaxUint32 {
			if relation != ugo_user {
				fmt.Println("Can't alter mtime. Not owner.")
				return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Permission denied."}, nil
			}
		}

		if len(newstat.Gid) != 0 {
			if info.n.Stat().Uid != c.uname ||
				!userInGroup(c.uname, newstat.Gid) {
				//fmt.Printf("uname: %s, gid: %s\n", c.uname, newstat.Gid)
				//fmt.Println("Can't changegroup. Not owner or not member of new group.")
				return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Permission denied."}, nil
			}
		}
	}

	// Do the changes.
	if len(newstat.Name) != 0 {
		stat.Name = newstat.Name
	}

	if newstat.Length != math.MaxUint64 {
		stat.Length = newstat.Length
	}

	if newstat.Mode != math.MaxUint32 {
		newmode := newstat.Mode & 0x000001FF
		stat.Mode = (stat.Mode & ^uint32(0x1FF)) | newmode
	}

	if newstat.Mtime != math.MaxUint32 {
		stat.Mtime = newstat.Mtime
	}

	if len(newstat.Gid) != 0 {
		stat.Gid = newstat.Gid
	}

	info.n.WriteStat(&stat)
	return &proto.RWstat{proto.Header{proto.Rwstat, t.Tag}}, nil

}
