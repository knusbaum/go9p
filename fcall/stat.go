package fcall

import (
	"fmt"
)

type TStat struct {
	FCall
	Fid uint32
}

func (stat *TStat) String() string {
	return fmt.Sprintf("tstat: [%s, fid: %d]", &stat.FCall, stat.Fid)
}

func (stat *TStat) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&stat.FCall, buff)
	if err != nil {
		return nil, err
	}

	stat.Fid, buff = FromLittleE32(buff)
	return buff, nil
}

func (stat *TStat) Compose() []byte {
	// size[4] Twrite tag[2] fid[4]
	length := 4 + 1 + 2 + 4
	buff := make([]byte, length)
	buffer := buff
	
	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = stat.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(stat.Tag, buffer)
	buffer = ToLittleE32(stat.Fid, buffer)
	
	return buff
}

func (stat *TStat) Reply(fs *Filesystem, conn *Connection, h Handler) IFCall {
	file := fs.FileForPath(conn.PathForFid(stat.Fid))
	if file == nil {
		return &RError{FCall{Rstat, stat.Tag}, "No such file."}
	}
	return &RStat{FCall{Rstat, stat.Tag}, file.stat}
}

type Stat struct {
	Stype uint16
	Dev uint32
	Qid Qid
	Mode uint32
	Atime uint32
	Mtime uint32
	Length uint64
	Name string
	Uid string
	Gid string
	Muid string
}

func (stat *Stat) String() string {
	return fmt.Sprintf("stype: %d, dev: %d, qid: [%s], mode: %o, atime: %d, mtime: %d, length: %d, name: %s, uid: %s, gid: %s, muid: %s",
		stat.Stype, stat.Dev, &stat.Qid, stat.Mode,
		stat.Atime, stat.Mtime, stat.Length, stat.Name, stat.Uid,
		stat.Gid, stat.Muid)
}

func (stat *Stat) Parse(buff []byte) ([]byte, error) {
	_, buff = FromLittleE16(buff) // throw away length
	stat.Stype, buff = FromLittleE16(buff)
	stat.Dev, buff = FromLittleE32(buff)
	buff, err := stat.Qid.Parse(buff)
	if err != nil {
		return nil, err
	}
	stat.Mode, buff = FromLittleE32(buff)
	stat.Atime, buff = FromLittleE32(buff)
	stat.Mtime, buff = FromLittleE32(buff)
	stat.Length, buff = FromLittleE64(buff)
	stat.Name, buff = FromString(buff)
	stat.Uid, buff = FromString(buff)
	stat.Gid, buff = FromString(buff)
	stat.Muid, buff = FromString(buff)
	return buff, nil
}

func (stat *Stat) ComposeLength() uint16 {
	// size[2], type[2], dev[4], qid[13], mode[4], atime[4], mtime[4], length[8],
	// name[s], uid[s], gid[s], muid[s]
	return uint16(2 + 2 + 4 + 13 + 4 + 4 + 4 + 8 +
		(2 + len(stat.Name)) +
		(2 + len(stat.Uid)) +
		(2 + len(stat.Gid)) +
		(2 + len(stat.Muid)))
}

func (stat *Stat) Compose() []byte {
	length := stat.ComposeLength()
	buff := make([]byte, length)
	buffer := buff

	buffer = ToLittleE16(length - 2, buffer)
	buffer = ToLittleE16(stat.Stype, buffer)
	buffer = ToLittleE32(stat.Dev, buffer)
	qidbuff := stat.Qid.Compose()
	copy(buffer, qidbuff)
	buffer = buffer[len(qidbuff):]
	buffer = ToLittleE32(stat.Mode, buffer)
	buffer = ToLittleE32(stat.Atime, buffer)
	buffer = ToLittleE32(stat.Mtime, buffer)
	buffer = ToLittleE64(stat.Length, buffer)
	buffer = ToString(stat.Name, buffer)
	buffer = ToString(stat.Uid, buffer)
	buffer = ToString(stat.Gid, buffer)
	buffer = ToString(stat.Muid, buffer)
	return buff
}

type RStat struct {
	FCall
	Stat
}

func (stat *RStat) String() string {
	return fmt.Sprintf("rstat: [%s, %s]",
		&stat.FCall, &stat.Stat)
}

func (stat *RStat) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&stat.FCall, buff)
	if err != nil {
		return nil, err
	}
	_, buff = FromLittleE16(buff) // stat length
	buff, err = stat.Stat.Parse(buff)
	if err != nil {
		return nil, err
	}
	return buff, nil
}

func (stat *RStat) Compose() []byte {
	// size[4] Rstat tag[2] stat[n]
	statLength := stat.Stat.ComposeLength()
	length := 4 + 1 + 2 + 2 + statLength
	buff := make([]byte, length)
	buffer := buff

	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = stat.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(stat.Tag, buffer)
	buffer = ToLittleE16(statLength, buffer)
	copy(buffer, stat.Stat.Compose())
	
	return buff
}
