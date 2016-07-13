package fcall

import (
	"fmt"
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

//func (wstat *TWstat) Reply(fs Filesystem, conn Connection) IFCall {
//	file := fs.FileForPath(conn.PathForFid(wstat.Fid))
//	if file == nil {
//		return &RError{FCall{Rerror, wstat.Tag}, "No such file."}
//	}
//
//	var stat *Stat
//	var newstat *Stat
//	stat = &file.stat
//	newstat = &wstat.Stat
//
//	// Need to implement a whole bunch of complicated rules.
//	// See: http://knusbaum.inlisp.org/res/rfc9p2000.html
//	
//	return &RError{FCall{Rerror, wstat.Tag}, "Not implemented."}
//}

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
