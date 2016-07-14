package fcall

import (
	"fmt"
)

type TWalk struct {
	FCall
	Fid uint32
	Newfid uint32
	Nwname uint16
	Wname []string
}

func (walk *TWalk) String() string {
	ret := fmt.Sprintf("twalk: [%s, fid: %d, newfid: %d, nwname: %d, wname: <",
		&walk.FCall, walk.Fid, walk.Newfid, walk.Nwname)
	for _, s := range walk.Wname {
		ret += s + ", "
	}
	ret += ">]"
	return ret
}

func (walk *TWalk) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&walk.FCall, buff)
	if err != nil {
		return nil, err
	}

	walk.Fid, buff = FromLittleE32(buff)
	walk.Newfid, buff = FromLittleE32(buff)
	walk.Nwname, buff = FromLittleE16(buff)
	walk.Wname = make([]string, walk.Nwname)
	var i uint16
	for ; i < walk.Nwname; i++ {
		walk.Wname[i], buff = FromString(buff)
	}
	return buff, nil
}

func (walk *TWalk) Compose() []byte {
	// size[4] Twalk  tag[2] fid[4] newfid[4] nwname[2] nwname*(wname[s])
	length := 4 + 1 + 2 + 4 + 4 + 2
	for _, name := range walk.Wname {
		length += 2 + len(name)
	}
	buff := make([]byte, length)
	buffer := buff

	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = walk.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(walk.Tag, buffer)
	buffer = ToLittleE32(walk.Fid, buffer)
	buffer = ToLittleE32(walk.Newfid, buffer)
	buffer = ToLittleE16(walk.Nwname, buffer)
	for _, name := range walk.Wname {
		buffer = ToString(name, buffer)
	}

	return buff
}

func (walk *TWalk) Reply(fs Filesystem, conn Connection) IFCall {
	file := fs.FileForPath(conn.PathForFid(walk.Fid))
	if file == nil {
		return &RWalk{FCall{Rwalk, walk.Tag}, 0, nil}
		//return &RError{FCall{walk.Ctype, walk.Tag}, "No such file."}
	}

	qids := make([]Qid, 0)
	path := file.path
	for i := 0; i < int(walk.Nwname); i++ {
		if path == "/" {
			path += walk.Wname[i]
		} else {
			path += "/" + walk.Wname[i]
		}
		currfile := fs.FileForPath(path)
		if currfile == nil {
			//return &RError{FCall{walk.Ctype, walk.Tag}, "No such path."}
			return &RWalk{FCall{Rwalk, walk.Tag}, 0, nil}
		}

		qids = append(qids, currfile.stat.Qid)
	}
	conn.SetFidPath(walk.Newfid, path)
	return &RWalk{FCall{Rwalk, walk.Tag}, uint16(len(qids)), qids}
}

type RWalk struct {
	FCall
	Nwqid uint16
	Wqid []Qid
}

func (walk *RWalk) String() string {
	ret := fmt.Sprintf("rwalk: [%s, nwqid: %d, wqid: <",
		&walk.FCall, walk.Nwqid)
	for _, qid := range walk.Wqid {
		ret += fmt.Sprintf("<%s>, ", &qid)
	}
	ret += ">]"
	return ret
}

func (walk *RWalk) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&walk.FCall, buff)
	if err != nil {
		return nil, err
	}

	walk.Nwqid, buff = FromLittleE16(buff)
	walk.Wqid = make([]Qid, walk.Nwqid)
	var i uint16
	for ; i < walk.Nwqid; i++ {
		buff, err = walk.Wqid[i].Parse(buff)
		if err != nil {
			return nil, err
		}
	}
	return buff, nil
}

func (walk *RWalk) Compose() []byte {
	// size[4] Rwalk tag[2] nwqid[2] nwqid*(wqid[13])
	length := 4 + 1 + 2 + 2 + (walk.Nwqid * 13)
	buff := make([]byte, length)
	buffer := buff

	fmt.Printf("Writing size: %d, type: %d, Tag: %d, Nwqid: %d, ",
		length, walk.Ctype, walk.Tag, walk.Nwqid)
	
	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = walk.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(walk.Tag, buffer)
	buffer = ToLittleE16(walk.Nwqid, buffer)
	for _, qid := range walk.Wqid {
		fmt.Printf("qid: %s", qid)
		qidbuff := qid.Compose()
		copy(buffer, qidbuff)
		buffer = buffer[len(qidbuff):]
	}
	fmt.Println("")
	
	return buff
}

func (walk *RWalk) Reply(fs Filesystem, conn Connection) IFCall {
	return nil
}
