package fcall

import (
	"fmt"
	"io"
)

const (
    Tversion = 100
    Rversion = 101
    Tauth = 102
    Rauth = 103
    Tattach = 104
    Rattach = 105
    Terror = 106 /* illegal */
    Rerror = 107
    Tflush = 108
    Rflush = 109
    Twalk = 110
    Rwalk = 111
    Topen = 112
    Ropen = 113
    Tcreate = 114
    Rcreate = 115
    Tread = 116
    Rread = 117
    Twrite = 118
    Rwrite = 119
    Tclunk = 120
    Rclunk = 121
    Tremove = 122
    Rremove = 123
    Tstat = 124
    Rstat = 125
    Twstat = 126
    Rwstat = 127
)

type IFCall interface {
	String() string
	Parse([]byte) ([]byte, error)
	Compose() []byte
	GetFCall() *FCall
}

type FCall struct {
	Ctype uint8
	Tag uint16
}

func (fc *FCall) String() string {
	return fmt.Sprintf("tag: %d", fc.Tag)
}

func (fc *FCall) GetFCall() *FCall {
	return fc
}

func fcParse(fc *FCall, buff []byte) ([]byte, error) {
	if len(buff) < 2 {
		return nil, &ParseError{fmt.Sprintf("expected 2 bytes. got: %d", len(buff))}
	}
	fc.Tag, buff = FromLittleE16(buff)
	return buff, nil
}

type Qid struct {
	Qtype uint8
	Vers uint32
	Uid uint64
}

func (qid *Qid) String() string {
	return fmt.Sprintf("qtype: %d, version: %d, uid: %d",
		qid.Qtype, qid.Vers, qid.Uid)
}

func (qid *Qid) Parse(buff []byte) ([]byte, error) {
	if len(buff) == 0 {
		return nil, &ParseError{"can't parse. Reached end of buffer."}
	}
	qid.Qtype = buff[0]
	qid.Vers, buff = FromLittleE32(buff[1:])
	qid.Uid, buff = FromLittleE64(buff)
	return buff, nil
}

func (qid *Qid) Compose() []byte {
	buff := make([]byte, 13)
	buffer := buff

	buffer[0] = qid.Qtype; buffer = buffer[1:]
	buffer = ToLittleE32(qid.Vers, buffer)
	buffer = ToLittleE64(qid.Uid, buffer)

	return buff
}

func ParseCall(r io.Reader) (IFCall, error) {
	if r == nil {
		return nil, &ParseError{"nil reader."}
	}

	sizebuff := make([]byte, 4)
	err := ReadBytes(r, sizebuff)
	if err != nil {
		return nil, err
	}
	// We now have the length of the call.
	length, _ := FromLittleE32(sizebuff)

	// Subtract 4 for uint32 length we read
	buff := make([]byte, length - 4)
	err = ReadBytes(r, buff)
	if err != nil {
		return nil, err
	}

	var ctype uint8 = buff[0]
	buff = buff[1:]

	var call IFCall;

	switch ctype {
	case Tversion:
		call = &TRVersion{}
		break
	case Rversion:
		call = &TRVersion{}
		break
	case Tauth:
		call = &TAuth{}
		break
	case Rauth:
		call = &RAuth{}
		break
	case Tattach:
		call = &TAttach{}
		break
	case Rattach:
		call = &RAttach{}
		break
	case Rerror:
		call = &RError{}
		break
	case Tflush:
		call = &TFlush{}
		break
	case Rflush:
		call = &RFlush{}
		break
	case Twalk:
		call = &TWalk{}
		break
	case Rwalk:
		call = &RWalk{}
		break
	case Topen:
		call = &TOpen{}
		break
	case Ropen:
		call = &ROpen{}
		break
	case Tcreate:
		call = &TCreate{}
		break
	case Rcreate:
		call = &RCreate{}
		break
	case Tread:
		call = &TRead{}
		break
	case Rread:
		call = &RRead{}
		break
	case Twrite:
		call = &TWrite{}
		break
	case Rwrite:
		call = &RWrite{}
		break
	case Tclunk:
		call = &TClunk{}
		break
	case Rclunk:
		call = &RClunk{}
		break
	case Tremove:
		call = &TRemove{}
		break
	case Rremove:
		call = &RRemove{}
		break
	case Tstat:
		call = &TStat{}
		break
	case Rstat:
		call = &RStat{}
		break
	case Twstat:
		call = &TWstat{}
		break
	case Rwstat:
		call = &RWstat{}
		break
	default:
		fmt.Println("No such case.")
	}

	call.Parse(buff)
	call.GetFCall().Ctype = ctype
	return call, nil
}