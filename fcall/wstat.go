package fcall

import (
	"fmt"
	"math"
)

type TWstat struct {
	FCall
	Fid uint32
	Stat Stat
}

func (wstat *TWstat) String() string {
	return fmt.Sprintf("twstat: [%s, fid: %d, %s]",
		&wstat.FCall, wstat.Fid, &wstat.Stat)
}

func (wstat *TWstat) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&wstat.FCall, buff)
	if err != nil {
		return nil, err
	}
	wstat.Fid, buff = FromLittleE32(buff)
	_, buff = FromLittleE16(buff) // Throw away stat length.
	buff, err = wstat.Stat.Parse(buff)
	if err != nil {
		return nil, err
	}
	return buff, nil
}

func (wstat *TWstat) Compose() []byte {
	// size[4] Twstat tag[2] fid[4] stat[n]
	statLength := wstat.Stat.ComposeLength()
	length := 4 + 1 + 2 + 4 + statLength
	buff := make([]byte, length)
	buffer := buff

	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = wstat.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(wstat.Tag, buffer)
	buffer = ToLittleE32(wstat.Fid, buffer)
	copy(buffer, wstat.Stat.Compose())

	return buff
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

func (wstat *TWstat) Reply(fs *Filesystem, conn *Connection) IFCall {
	file := fs.FileForPath(conn.PathForFid(wstat.Fid))
	if file == nil {
		return &RError{FCall{Rerror, wstat.Tag}, "No such file."}
	}

	var stat *Stat
	var newstat *Stat
	stat = &file.stat
	newstat = &wstat.Stat

	relation := UserRelation(conn.uname, file)

	{
		// Need to check all this stuff before we change *ANYTHING*
		// The server needs to accept ALL the changes or none of them.
		if len(newstat.Name) != 0 {
			if relation != ugo_user {
				fmt.Println("Can't change name. Not owner.")
				return &RError{FCall{Rerror, wstat.Tag}, "Permission denied."}
			}
		}

		if newstat.Length != math.MaxUint64 {
			if !OpenPermission(conn.uname, file, Owrite) {
				fmt.Println("Can't alter length. Don't have write permission.")
				return &RError{FCall{Rerror, wstat.Tag}, "Permission denied."}
			}
		}

		if newstat.Mode != math.MaxUint32 {
			if relation != ugo_user {
				fmt.Println("Can't alter mode. Not owner.")
				return &RError{FCall{Rerror, wstat.Tag}, "Permission denied."}
			}
		}

		if newstat.Mtime != math.MaxUint32 {
			if relation != ugo_user {
				fmt.Println("Can't alter mtime. Not owner.")
				return &RError{FCall{Rerror, wstat.Tag}, "Permission denied."}
			}
		}

		if len(newstat.Gid) != 0 {
			if file.stat.Uid != conn.uname ||
				!UserInGroup(conn.uname, newstat.Gid) {
				fmt.Println("Can't changegroup. Not owner or not member of new group.")
				return &RError{FCall{Rerror, wstat.Tag}, "Permission denied."}
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
		newmode := newstat.Mode & 0x000001FF;
		stat.Mode = (stat.Mode & ^uint32(0x1FF)) | newmode
	}

	if newstat.Mtime != math.MaxUint32 {
		stat.Mtime = newstat.Mtime
	}

	if len(newstat.Gid) != 0 {
		stat.Gid = newstat.Gid
	}

	return &RWstat{FCall{Rwstat, wstat.Tag}}
}

type RWstat struct {
	FCall
}

func (wstat *RWstat) String() string {
	return fmt.Sprintf("rwstat: [%s]", &wstat.FCall)
}

func (wstat *RWstat) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&wstat.FCall, buff)
	if err != nil {
		return nil, err
	}

	return buff, nil
}

func (wstat *RWstat) Compose() []byte {
	// size[4] Rwstat tag[2]
	length := 4 + 1 + 2
	buff := make([]byte, length)
	buffer := buff

	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = wstat.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(wstat.Tag, buffer)
	return buff
}
