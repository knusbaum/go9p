package proto

import (
	"fmt"
	"io"
)

// These constants represent the message types and belong
// in the type field of the Header that is a part of every
// FCall.
const (
	Tversion = 100
	Rversion = 101
	Tauth    = 102
	Rauth    = 103
	Tattach  = 104
	Rattach  = 105
	Terror   = 106 /* illegal */
	Rerror   = 107
	Tflush   = 108
	Rflush   = 109
	Twalk    = 110
	Rwalk    = 111
	Topen    = 112
	Ropen    = 113
	Tcreate  = 114
	Rcreate  = 115
	Tread    = 116
	Rread    = 117
	Twrite   = 118
	Rwrite   = 119
	Tclunk   = 120
	Rclunk   = 121
	Tremove  = 122
	Rremove  = 123
	Tstat    = 124
	Rstat    = 125
	Twstat   = 126
	Rwstat   = 127
)

const (
	maxMsgLen = 65535 // 65k should be enough for anyone.
)

// FCall - the interface that all FCall types imlement.
// String - typical human readable string representation.
// Compose - returns a slice containing the call serialized
// according the the 9P2000 protocol, ready to be written.
type FCall interface {
	String() string
	Compose() []byte
	parse([]byte) ([]byte, error)
}

// Header - every 9p message begins with this header.
type Header struct {
	Type uint8
	Tag  uint16
}

func (fc *Header) String() string {
	return fmt.Sprintf("tag: %d", fc.Tag)
}

func (c *Header) parse(buff []byte) ([]byte, error) {
	if len(buff) < 3 {
		return nil, &ParseError{fmt.Sprintf("expected 2 bytes. got: %d", len(buff))}
	}
	c.Type = buff[0]
	buff = buff[1:]
	c.Tag, buff = fromLittleE16(buff)
	return buff, nil
}

// Qid - Qids are unique ids for files. Qtype should be the upper
// 8 bits of the file's permissions (Stat.Mode)
type Qid struct {
	Qtype uint8
	Vers  uint32
	Uid   uint64
}

func (qid *Qid) String() string {
	return fmt.Sprintf("qtype: 0x%X, version: %d, uid: %d",
		qid.Qtype, qid.Vers, qid.Uid)
}

// parse - parse a Qid from a slice of a 9P2000 stream
func (qid *Qid) parse(buff []byte) ([]byte, error) {
	if len(buff) == 0 {
		return nil, &ParseError{"can't parse. Reached end of buffer."}
	}
	qid.Qtype = buff[0]
	qid.Vers, buff = fromLittleE32(buff[1:])
	qid.Uid, buff = fromLittleE64(buff)
	return buff, nil
}

// Compose - Returns a slice of the Qid serialized to be
// written out on a 9P2000 stream.
func (qid *Qid) Compose() []byte {
	buff := make([]byte, 13)
	buffer := buff

	buffer[0] = qid.Qtype
	buffer = buffer[1:]
	buffer = toLittleE32(qid.Vers, buffer)
	buffer = toLittleE64(qid.Uid, buffer)

	return buff
}

// ParseCall - Reads from a 9P2000 stream and parses an FCall from it.
// On error, the protocol on the stream is in an unknown state and
// the stream should be closed.
func ParseCall(r io.Reader) (FCall, error) {
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
	if length > maxMsgLen {
		return nil, fmt.Errorf("Can't allocate %d bytes for message.", length)
	}

	// Subtract 4 for uint32 length we read
	buff := make([]byte, length-4)
	err = readBytes(r, buff)
	if err != nil {
		return nil, err
	}

	var h Header
	buff, err = h.parse(buff)
	if err != nil {
		return nil, err
	}

	var fc FCall

	switch h.Type {
	case Tversion:
		fc = &TRVersion{Header: h}
		break
	case Rversion:
		fc = &TRVersion{Header: h}
		break
	case Tauth:
		fc = &TAuth{Header: h}
		break
	case Rauth:
		fc = &RAuth{Header: h}
		break
	case Tattach:
		fc = &TAttach{Header: h}
		break
	case Rattach:
		fc = &RAttach{Header: h}
		break
	case Rerror:
		fc = &RError{Header: h}
		break
	case Tflush:
		fc = &TFlush{Header: h}
		break
	case Rflush:
		fc = &RFlush{Header: h}
		break
	case Twalk:
		fc = &TWalk{Header: h}
		break
	case Rwalk:
		fc = &RWalk{Header: h}
		break
	case Topen:
		fc = &TOpen{Header: h}
		break
	case Ropen:
		fc = &ROpen{Header: h}
		break
	case Tcreate:
		fc = &TCreate{Header: h}
		break
	case Rcreate:
		fc = &RCreate{Header: h}
		break
	case Tread:
		fc = &TRead{Header: h}
		break
	case Rread:
		fc = &RRead{Header: h}
		break
	case Twrite:
		fc = &TWrite{Header: h}
		break
	case Rwrite:
		fc = &RWrite{Header: h}
		break
	case Tclunk:
		fc = &TClunk{Header: h}
		break
	case Rclunk:
		fc = &RClunk{Header: h}
		break
	case Tremove:
		fc = &TRemove{Header: h}
		break
	case Rremove:
		fc = &RRemove{Header: h}
		break
	case Tstat:
		fc = &TStat{Header: h}
		break
	case Rstat:
		fc = &RStat{Header: h}
		break
	case Twstat:
		fc = &TWstat{Header: h}
		break
	case Rwstat:
		fc = &RWstat{Header: h}
		break
	default:
		return nil, &ParseError{fmt.Sprintf("Message type %d not implemented.", h.Type)}
	}

	_, err = fc.parse(buff)
	if err != nil {
		return nil, err
	}
	return fc, nil
}
