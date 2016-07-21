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

// IFCall - the interface that all FCall-like types imlement.
// String - typical human readable string representation.
// Parse - Parse the call from a slice.
// Compose - returns a slice containing the call serialized
// according the the 9P2000 protocol, ready to be written out
// to a client.
// GetFCall() - get the base FCall associated with this
// call.
// Reply() - Handles the fcall and calls appropriate functions
// in Server. Returns nil or an IFCall that should be sent as a
// response back to the client.
type IFCall interface {
	String() string
	Parse([]byte) ([]byte, error)
	Compose() []byte
	GetFCall() *FCall
	Reply(*filesystem, *connection, *Server) IFCall
}

// FCall - The base FCall type. All FCall-like types embed this
// and inherit its functions.
// For explanations of the functions, see IFCall.
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

// ParseCall - Reads from a 9P2000 stream and parses an IFCall from it.
// On error, the protocol on the stream is in an unknown state and
// the stream should be closed.
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
	case tflush:
		call = &TFlush{}
		break
	case rflush:
		call = &RFlush{}
		break
	case twalk:
		call = &TWalk{}
		break
	case rwalk:
		call = &RWalk{}
		break
	case topen:
		call = &TOpen{}
		break
	case ropen:
		call = &ROpen{}
		break
	case tcreate:
		call = &TCreate{}
		break
	case rcreate:
		call = &RCreate{}
		break
	case tread:
		call = &TRead{}
		break
	case rread:
		call = &RRead{}
		break
	case twrite:
		call = &TWrite{}
		break
	case rwrite:
		call = &RWrite{}
		break
	case tclunk:
		call = &TClunk{}
		break
	case rclunk:
		call = &RClunk{}
		break
	case tremove:
		call = &TRemove{}
		break
	case rremove:
		call = &RRemove{}
		break
	case tstat:
		call = &TStat{}
		break
	case rstat:
		call = &RStat{}
		break
	case twstat:
		call = &TWstat{}
		break
	case rwstat:
		call = &RWstat{}
		break
	default:
		tag, _ := fromLittleE16(buff)
		return &RError{FCall{rerror, tag}, "Not Implemented."},
			&ParseError{fmt.Sprintf("Not implemented: %d", ctype)}
	}

	call.Parse(buff)
	call.GetFCall().Ctype = ctype
	return call, nil
}
