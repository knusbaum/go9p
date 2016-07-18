package fcall

import (
	"fmt"
)

type TAttach struct {
	FCall
	Fid uint32
	Afid uint32
	Uname string
	Aname string
}

func (attach *TAttach) String() string {
	return fmt.Sprintf("tattach: [%s, fid: %d, afid: %d, uname: %s, aname: %s]",
		&attach.FCall, attach.Fid, attach.Afid, attach.Uname, attach.Aname)
}

func (attach *TAttach) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&attach.FCall, buff)
	if err != nil {
		return nil, err
	}

	attach.Fid, buff = FromLittleE32(buff)
	attach.Afid, buff = FromLittleE32(buff)
	attach.Uname, buff = FromString(buff)
	attach.Aname, buff = FromString(buff)
	return buff, nil
}

func (attach *TAttach) Compose() []byte {
	// size[4] Tattach tag[2] fid[4] afid[4] uname[s] aname[s]
	length := 4 + 1 + 2 + 4 + 4 +
		(2 + len(attach.Uname)) + (2 + len(attach.Aname))
	buff := make([]byte, length)
	buffer := buff

	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = attach.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(attach.Tag, buffer)
	buffer = ToLittleE32(attach.Fid, buffer)
	buffer = ToLittleE32(attach.Afid, buffer)
	buffer = ToString(attach.Uname, buffer)
	buffer = ToString(attach.Aname, buffer)
	return buff
}

func (attach *TAttach) Reply(filesystem *Filesystem, conn *Connection, h Handler) IFCall {
	conn.SetUname(attach.Uname)
	conn.SetFidPath(attach.Fid, "/")
	reply := RAttach{
		FCall: FCall{
			Ctype: Rattach,
			Tag: attach.Tag},
		Qid: Qid{(1 << 7), 28, 1}}
	return &reply
}

type RAttach struct {
	FCall
	Qid Qid
}

func (attach *RAttach) String() string {
	return fmt.Sprintf("rattach: [%s, qid: [%s]]",
		&attach.FCall, &attach.Qid)
}

func (attach *RAttach) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&attach.FCall, buff)
	if err != nil {
		return nil, err
	}

	buff, err = attach.Qid.Parse(buff)
	if err != nil {
		return nil, err
	}
	return buff, nil
}

func (attach *RAttach) Compose() []byte {
	// size[4] Rattach tag[2] qid[13]
	length := 4 + 1 + 2 + 13
	buff := make([]byte, length)
	buffer := buff

	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = attach.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(attach.Tag, buffer)
	qidbuff := attach.Qid.Compose()
	copy(buffer, qidbuff)
	return buff
}
