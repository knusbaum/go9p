package server

import (
	"fmt"
	"math"
	"reflect"

	"github.com/knusbaum/go9p2/fs"
	"github.com/knusbaum/go9p2/proto"
)

type fidInfo struct {
	n          fs.FSNode
	openMode   proto.Mode
	openOffset uint64
	extra      interface{}
}

func NewFidInfo(n fs.FSNode) *fidInfo {
	return &fidInfo{
		n:        n,
		openMode: proto.None,
	}
}

type conn struct {
	fs    *fs.FS
	uname string
	fids  map[uint32]*fidInfo
}

func (c *conn) handleCall(call proto.FCall) (proto.FCall, error) {
	switch call.(type) {
	case *proto.TRVersion:
		return c.handleTVersion(call.(*proto.TRVersion))
	case *proto.TAuth:
		return c.handleTAuth(call.(*proto.TAuth))
	case *proto.TAttach:
		return c.handleTAttach(call.(*proto.TAttach))
	case *proto.TFlush:
		return c.handleTFlush(call.(*proto.TFlush))
	case *proto.TWalk:
		return c.handleTWalk(call.(*proto.TWalk))
	case *proto.TOpen:
		return c.handleTOpen(call.(*proto.TOpen))
	case *proto.TCreate:
		return c.handleTCreate(call.(*proto.TCreate))
	case *proto.TRead:
		return c.handleTRead(call.(*proto.TRead))
	case *proto.TWrite:
		return c.handleTWrite(call.(*proto.TWrite))
	case *proto.TClunk:
		return c.handleTClunk(call.(*proto.TClunk))
	case *proto.TRemove:
		return c.handleTRemove(call.(*proto.TRemove))
	case *proto.TStat:
		return c.handleTStat(call.(*proto.TStat))
	case *proto.TWstat:
		return c.handleTWstat(call.(*proto.TWstat))
	default:
		return nil, fmt.Errorf("Invalid call: %s", reflect .TypeOf(call))
	}
}

func (c *conn) handleTVersion(t *proto.TRVersion) (proto.FCall, error) {
	var reply proto.TRVersion
	if t.Type == proto.Tversion {
		reply = *t
		reply.Type = proto.Rversion
		return &reply, nil
	} else {
		return nil, fmt.Errorf("Cannot reply to type %d\n", t.Type)
	}
}

func (c *conn) handleTAuth(t *proto.TAuth) (proto.FCall, error) {
	return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Authentication Not Supported."}, nil
}

func (c *conn) handleTAttach(t *proto.TAttach) (proto.FCall, error) {
	c.uname = t.Uname
	c.fids[t.Fid] = NewFidInfo(c.fs.Root)
	return &proto.RAttach{proto.Header{proto.Rattach, t.Tag}, c.fs.Root.Stat().Qid}, nil
}

func (c *conn) handleTFlush(t *proto.TFlush) (proto.FCall, error) {
	return &proto.RFlush{proto.Header{proto.Rflush, t.Tag}}, nil
}

func (c *conn) handleTWalk(t *proto.TWalk) (proto.FCall, error) {
	info, ok := c.fids[t.Fid]
	if !ok {
		return &proto.RError{proto.Header{t.Type, t.Tag}, "Bad Fid."}, nil
	}
	file := info.n
	if t.Nwname > 0 && t.Wname[0] == ".." {
		parent := file.Parent()
		if parent != nil {
			c.fids[t.Newfid] = NewFidInfo(parent)
			qids := make([]proto.Qid, 1)
			qids[0] = parent.Stat().Qid
			return &proto.RWalk{proto.Header{proto.Rwalk, t.Tag}, 1, qids}, nil
		} else {
			return &proto.RWalk{proto.Header{proto.Rwalk, t.Tag}, 0, nil}, nil
		}
	}

	qids := make([]proto.Qid, 0)
	for i := 0; i < int(t.Nwname); i++ {
		if dir, ok := file.(fs.Dir); ok {
			file, ok = dir.Children()[t.Wname[i]]
			if !ok {
				if c.fs.WalkFail == nil {
					return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "No such path.1"}, nil
				}
				f, err := c.fs.WalkFail(c.fs, dir, t.Wname[i])
				if err != nil {
					return &proto.RError{proto.Header{proto.Rerror, t.Tag}, err.Error()}, nil
				}
				if f == nil {
					return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "No such path.2"}, nil
				}
				dir.AddChild(f)
				file = f
//				
//				if c.fs.WalkFail != nil {
//					f := c.fs.WalkFail(c.fs, dir, t.Wname[i])
//					if f != nil {
//						file = f
//					}
//				}
//				//fmt.Printf("Can't find [%s] in [%s]: %v\n", t.Wname[i], dir.Stat().Name, t.Wname[i])
//				return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "No such path.1"}, nil
			}
			qids = append(qids, file.Stat().Qid)
		} else {
			return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "No such path.3"}, nil
		}
	}
	c.fids[t.Newfid] = NewFidInfo(file)
	return &proto.RWalk{proto.Header{proto.Rwalk, t.Tag}, uint16(len(qids)), qids}, nil
}

func (c *conn) handleTOpen(t *proto.TOpen) (proto.FCall, error) {
	info, ok := c.fids[t.Fid]
	if !ok {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Bad Fid."}, nil
	}
	if info.openMode != proto.None {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Fid already open."}, nil
	}
	if !fs.OpenPermission(info.n, c.uname, uint8(t.Mode)&0x0F) {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Permission denied."}, nil
	}

	switch n := info.n.(type) {
	case fs.Dir:
		if (t.Mode&0x0F) == proto.Owrite ||
			(t.Mode&0x0F) == proto.Ordwr {
			return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Cannot write to directory."}, nil
		}
		children := n.Children()
		cl := make([]fs.FSNode, 0)
		for _, c := range children {
			cl = append(cl, c)
		}
		info.extra = cl
	case fs.File:
		err := n.Open(t.Fid, uint8(t.Mode))
		if err != nil {
			return &proto.RError{proto.Header{proto.Rerror, t.Tag}, err.Error()}, nil
		}
	}
	info.openMode = t.Mode
	info.openOffset = info.n.Stat().Length
	
	return &proto.ROpen{proto.Header{proto.Ropen, t.Tag}, info.n.Stat().Qid, proto.IOUnit}, nil
}

func (c *conn) handleTCreate(t *proto.TCreate) (proto.FCall, error) {
	info, ok := c.fids[t.Fid]
	if !ok {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Bad Fid."}, nil
	}
	if !fs.OpenPermission(info.n, c.uname, proto.Owrite) {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Permission denied."}, nil
	}

	if dir, ok := info.n.(fs.Dir); ok {
		var new fs.FSNode
		var err error
		if t.Perm & proto.DMDIR != 0 {
			if c.fs.CreateDir != nil {
				new, err = c.fs.CreateDir(c.fs, dir, c.uname, t.Name, t.Perm, t.Mode)
			} else {
				err = fmt.Errorf("Cannot create directories.")
			}
		} else {
			if c.fs.CreateFile != nil {
				new, err = c.fs.CreateFile(c.fs, dir, c.uname, t.Name, t.Perm, t.Mode)
			} else {
				err = fmt.Errorf("Cannot create files.")
			}
		}
		if err != nil {
			return &proto.RError{proto.Header{proto.Rerror, t.Tag}, err.Error()}, nil
		}
		info = NewFidInfo(new)
		info.openMode = proto.Mode(t.Mode)
		info.openOffset = 0
		c.fids[t.Fid] = info
		if f, ok := new.(fs.File); ok {
			err := f.Open(t.Fid, t.Mode)
			if err != nil {
				return &proto.RError{proto.Header{proto.Rerror, t.Tag}, err.Error()}, nil
			}
		}
		return &proto.RCreate{proto.Header{proto.Rcreate, t.Tag}, new.Stat().Qid, proto.IOUnit}, nil
	} else if f, ok := info.n.(fs.File); ok {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, f.Stat().Name + ": IS A FILE Not a directory"}, nil
	} else {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, info.n.Stat().Name + ": Not a directory"}, nil
	}
}

func (c *conn) handleTRead(t *proto.TRead) (proto.FCall, error) {
	if t.Count > proto.IOUnit {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Read size too large."}, nil
	}
	info, ok := c.fids[t.Fid]
	if !ok {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Bad Fid."}, nil
	}

	openmode := info.openMode & 0x0F
	// TODO: Can't we just check against None?
	if openmode != proto.Oread &&
		openmode != proto.Ordwr &&
		openmode != proto.Oexec {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "File not opened."}, nil
	}

	switch n := info.n.(type) {
	case fs.File:
		data, err := n.Read(t.Fid, t.Offset, uint64(t.Count))
		if err != nil {
			return &proto.RError{proto.Header{proto.Rerror, t.Tag}, err.Error()}, nil
		}
		return &proto.RRead{proto.Header{proto.Rread, t.Tag}, uint32(len(data)), data}, nil
	case fs.Dir:
		return readDir(t, info), nil
	}
	return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "File not opened."}, nil
}

func readDir(t *proto.TRead, info *fidInfo) proto.FCall {
	contents := make([]byte, 0)
	children := info.extra.([]fs.FSNode)

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

func (c *conn) handleTWrite(t *proto.TWrite) (proto.FCall, error) {
	info, ok := c.fids[t.Fid]
	if !ok {
		// TODO: Handle Auth
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Bad Fid."}, nil
	}
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

	if f, ok := info.n.(fs.File); ok {
		n, err := f.Write(t.Fid, offset, t.Data)
		if err != nil {
			return &proto.RError{proto.Header{proto.Rerror, t.Tag}, err.Error()}, nil
		}
		return &proto.RWrite{proto.Header{proto.Rwrite, t.Tag}, n}, nil
	} else {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Cannot write to directory."}, nil
	}
}

func (c *conn) handleTClunk(t *proto.TClunk) (proto.FCall, error) {
	info, ok := c.fids[t.Fid]
	delete(c.fids, t.Fid)
	if !ok {
		return &proto.RClunk{proto.Header{proto.Rclunk, t.Tag}}, nil
	}
	if info.openMode != proto.None {
		if f, ok := info.n.(fs.File); ok {
			err := f.Close(t.Fid)
			if err != nil {
				return &proto.RError{proto.Header{proto.Rerror, t.Tag}, err.Error()}, nil
			}
		}
	}
	return &proto.RClunk{proto.Header{proto.Rclunk, t.Tag}}, nil
}

func (c *conn) handleTRemove(t *proto.TRemove) (proto.FCall, error) {
	info, ok := c.fids[t.Fid]
	if !ok {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Bad Fid."}, nil
	}
	if !fs.OpenPermission(info.n, c.uname, proto.Owrite) {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Permission denied."}, nil
	}

	var err error
	if c.fs.RemoveFile != nil {
		err = c.fs.RemoveFile(c.fs, info.n)
	} else {
		err = fmt.Errorf("Cannot delete files.")
	}
	if err != nil {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, err.Error()}, nil
	}
	return &proto.RRemove{proto.Header{proto.Rremove, t.Tag}}, nil
}

func (c *conn) handleTStat(t *proto.TStat) (proto.FCall, error) {
	info, ok := c.fids[t.Fid]
	if !ok {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Bad Fid."}, nil
	}
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
func (c *conn) handleTWstat(t *proto.TWstat) (proto.FCall, error) {
	info, ok := c.fids[t.Fid]
	if !ok {
		return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Bad Fid."}, nil
	}

	stat := info.n.Stat()
	newstat := &t.Stat
	relation := fs.UserRelation(c.uname, info.n)

	{
		// Need to check all this stuff before we change *ANYTHING*
		// The server needs to accept ALL the changes or none of them.
		if len(newstat.Name) != 0 {
			if relation != fs.UGO_user {
				fmt.Println("Can't change name. Not owner.")
				return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Permission denied."}, nil
			}
		}

		if newstat.Length != math.MaxUint64 {
			if !fs.OpenPermission(info.n, c.uname, proto.Owrite) {
				fmt.Println("Can't alter length. Don't have write permission.")
				return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Permission denied."}, nil
			}
		}

		if newstat.Mode != math.MaxUint32 {
			if relation != fs.UGO_user {
				fmt.Println("Can't alter mode. Not owner.")
				return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Permission denied."}, nil
			}
		}

		if newstat.Mtime != math.MaxUint32 {
			if relation != fs.UGO_user {
				fmt.Println("Can't alter mtime. Not owner.")
				return &proto.RError{proto.Header{proto.Rerror, t.Tag}, "Permission denied."}, nil
			}
		}

		if len(newstat.Gid) != 0 {
			if info.n.Stat().Uid != c.uname ||
				!fs.UserInGroup(c.uname, newstat.Gid) {
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
