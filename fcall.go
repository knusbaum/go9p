package go9p

import (
	"fmt"
	"io"
)

const (
	tversion = 100
	rversion = 101
	tauth    = 102
	rauth    = 103
	tattach  = 104
	rattach  = 105
	terror   = 106 /* illegal */
	rerror   = 107
	tflush   = 108
	rflush   = 109
	twalk    = 110
	rwalk    = 111
	topen    = 112
	ropen    = 113
	tcreate  = 114
	rcreate  = 115
	tread    = 116
	rread    = 117
	twrite   = 118
	rwrite   = 119
	tclunk   = 120
	rclunk   = 121
	tremove  = 122
	rremove  = 123
	tstat    = 124
	rstat    = 125
	twstat   = 126
	rwstat   = 127
)

type IFCall interface {
	String() string
	Parse([]byte) ([]byte, error)
	Compose() []byte
	GetFCall() *FCall
	Reply(*filesystem, *connection, *Server) IFCall
}

type FCall struct {
	Ctype uint8
	Tag   uint16
}

func (fc *FCall) String() string {
	return fmt.Sprintf("tag: %d", fc.Tag)
}

func (fc *FCall) GetFCall() *FCall {
	return fc
}

func (fc *FCall) Reply(fs *filesystem, conn *connection, s *Server) IFCall {
	return nil
}

func fcParse(fc *FCall, buff []byte) ([]byte, error) {
	if len(buff) < 2 {
		return nil, &ParseError{fmt.Sprintf("expected 2 bytes. got: %d", len(buff))}
	}
	fc.Tag, buff = fromLittleE16(buff)
	return buff, nil
}

func ParseCall(r io.Reader) (IFCall, error) {
	if r == nil {
		return nil, &ParseError{"nil reader."}
	}

	sizebuff := make([]byte, 4)
	err := readBytes(r, sizebuff)
	if err != nil {
		return nil, err
	}
	// We now have the length of the call.
	length, _ := fromLittleE32(sizebuff)

	// Subtract 4 for uint32 length we read
	buff := make([]byte, length-4)
	err = readBytes(r, buff)
	if err != nil {
		return nil, err
	}

	var ctype uint8 = buff[0]
	buff = buff[1:]

	var call IFCall

	switch ctype {
	case tversion:
		call = &TRVersion{}
		break
	case rversion:
		call = &TRVersion{}
		break
	case tauth:
		call = &TAuth{}
		break
	case rauth:
		call = &RAuth{}
		break
	case tattach:
		call = &TAttach{}
		break
	case rattach:
		call = &RAttach{}
		break
	case rerror:
		call = &RError{}
		break
		//	case Tflush:
		//		call = &TFlush{}
		//		break
		//	case Rflush:
		//		call = &RFlush{}
		//		break
	case twalk:
		call = &TWalk{}
		break
		//	case Rwalk:
		//		call = &RWalk{}
		//		break
	case topen:
		call = &TOpen{}
		break
		//	case Ropen:
		//		call = &ROpen{}
		//		break
	case tcreate:
		call = &TCreate{}
		break
		//	case Rcreate:
		//		call = &RCreate{}
		//		break
	case tread:
		call = &TRead{}
		break
		//	case Rread:
		//		call = &RRead{}
		//		break
	case twrite:
		call = &TWrite{}
		break
		//	case Rwrite:
		//		call = &RWrite{}
		//		break
	case tclunk:
		call = &TClunk{}
		break
		//	case Rclunk:
		//		call = &RClunk{}
		//		break
	case tremove:
		call = &TRemove{}
		break
		//	case Rremove:
		//		call = &RRemove{}
		//		break
	case tstat:
		call = &TStat{}
		break
		//	case Rstat:
		//		call = &RStat{}
		//		break
	case twstat:
		call = &TWstat{}
		break
		//	case Rwstat:
		//		call = &RWstat{}
		//		break
	default:
		tag, _ := fromLittleE16(buff)
		return &RError{FCall{rerror, tag}, "Not Implemented."},
			&ParseError{fmt.Sprintf("Not implemented: %d", ctype)}
	}

	call.Parse(buff)
	call.GetFCall().Ctype = ctype
	return call, nil
}
