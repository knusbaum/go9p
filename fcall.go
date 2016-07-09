package go9p

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
}

type FCall struct {
	tag uint16
}

func (fc *FCall) String() string {
	return fmt.Sprintf("tag: %d", fc.tag)
}

func (fc *FCall) Parse(buff []byte) ([]byte, error) {
	if len(buff) < 2 {
		return nil, &ParseError{fmt.Sprintf("expected 2 bytes. got: %d", len(buff))}
	}
	fc.tag, buff = FromLittleE16(buff)
	return buff, nil
}

type Qid struct {
	qtype uint8
	vers uint32
	uid uint64
}

func (qid *Qid) String() string {
	return fmt.Sprintf("qtype: %d, version: %d, uid: %d",
		qid.qtype, qid.vers, qid.uid)
}

func (qid *Qid) Parse(buff []byte) ([]byte, error) {
	if len(buff) == 0 {
		return nil, &ParseError{"can't parse. Reached end of buffer."}
	}
	qid.qtype = buff[0]
	qid.vers, buff = FromLittleE32(buff[1:])
	qid.uid, buff = FromLittleE64(buff)
	return buff, nil
}

type TRVersion struct {
	FCall
	msize uint32
	version string
}

func (version *TRVersion) String() string {
	return fmt.Sprintf("(t|r)version: [%s, msize: %d, version: %s]",
		&version.FCall, version.msize, version.version)
}

func (version *TRVersion) Parse(buff []byte) ([]byte, error) {
	buff, err := version.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	version.msize, buff = FromLittleE32(buff)
	version.version, buff = FromString(buff)
	return buff, nil
}

type TAuth struct {
	FCall
	afid uint32
	uname string
	aname string
}

func (auth *TAuth) String() string {
	return fmt.Sprintf("tauth: [%s, afid: %d, uname: %s, aname: %s]",
		&auth.FCall, auth.afid, auth.uname, auth.aname)
}

func (auth *TAuth) Parse(buff []byte) ([]byte, error) {
	buff, err := auth.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	auth.afid, buff = FromLittleE32(buff)
	auth.uname, buff = FromString(buff)
	auth.aname, buff = FromString(buff)
	return buff, nil
}

type RAuth struct {
	FCall
	aqid Qid
}

func (auth *RAuth) String() string {
	return fmt.Sprintf("rauth: [%s, aqid: [%s]]",
		&auth.FCall, &auth.aqid)
}

func (auth *RAuth) Parse(buff []byte) ([]byte, error) {
	buff, err := auth.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	buff, err = auth.aqid.Parse(buff)
	if err != nil {
		return nil, err
	}
	return buff, nil
}

type RError struct {
	FCall
	ename string
}

func (error *RError) String() string {
	return fmt.Sprintf("rerror: [%s, ename: %s]",
		&error.FCall, error.ename)
}

func (error *RError) Parse(buff []byte) ([]byte, error) {
	buff, err := error.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	error.ename, buff = FromString(buff)
	return buff, nil
}

type TFlush struct {
	FCall
	oldtag uint16
}

func (flush *TFlush) String() string {
	return fmt.Sprintf("tflush: [%s, oldtag: %d]",
		&flush.FCall, flush.oldtag)
}

func (flush *TFlush) Parse(buff []byte) ([]byte, error) {
	buff, err := flush.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	flush.oldtag, buff = FromLittleE16(buff)
	return buff, nil
}

type RFlush struct {
	FCall
}

func (flush *RFlush) String() string {
	return fmt.Sprintf("rflush: [%s]", &flush.FCall)
}

func (flush *RFlush) Parse(buff []byte) ([]byte, error) {
	buff, err := flush.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	return buff, nil
}

type TAttach struct {
	FCall
	fid uint32
	afid uint32
	uname string
	aname string
}

func (attach *TAttach) String() string {
	return fmt.Sprintf("tattach: [%s, fid: %d, afid: %d, uname: %s, aname: %s]",
		&attach.FCall, attach.fid, attach.afid, attach.uname, attach.aname)
}

func (attach *TAttach) Parse(buff []byte) ([]byte, error) {
	buff, err := attach.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	attach.fid, buff = FromLittleE32(buff)
	attach.afid, buff = FromLittleE32(buff)
	attach.uname, buff = FromString(buff)
	attach.aname, buff = FromString(buff)
	return buff, nil
}

type RAttach struct {
	FCall
	qid Qid
}

func (attach *RAttach) String() string {
	return fmt.Sprintf("rattach: [%s, qid: [%s]]",
		&attach.FCall, &attach.qid)
}

func (attach *RAttach) Parse(buff []byte) ([]byte, error) {
	buff, err := attach.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	buff, err = attach.qid.Parse(buff)
	if err != nil {
		return nil, err
	}
	return buff, nil
}

type TWalk struct {
	FCall
	fid uint32
	newfid uint32
	nwname uint16
	wname []string
}

func (walk *TWalk) String() string {
	ret := fmt.Sprintf("twalk: [%s, fid: %d, newfid: %d, nwname: %d, wname: <",
		&walk.FCall, walk.fid, walk.newfid, walk.nwname)
	for _, s := range walk.wname {
		ret += s + ", "
	}
	ret += ">]"
	return ret
}

func (walk *TWalk) Parse(buff []byte) ([]byte, error) {
	buff, err := walk.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	walk.fid, buff = FromLittleE32(buff)
	walk.newfid, buff = FromLittleE32(buff)
	walk.nwname, buff = FromLittleE16(buff)
	walk.wname = make([]string, walk.nwname)
	var i uint16
	for ; i < walk.nwname; i++ {
		walk.wname[i], buff = FromString(buff)
	}
	return buff, nil
}

type RWalk struct {
	FCall
	nwqid uint16
	wqid []Qid
}

func (walk *RWalk) String() string {
	ret := fmt.Sprintf("rwalk: [%s, nwqid: %d, wqid: <",
		&walk.FCall, walk.nwqid)
	for _, qid := range walk.wqid {
		ret += fmt.Sprintf("%s, ", &qid)
	}
	ret += ">]"
	return ret
}

func (walk *RWalk) Parse(buff []byte) ([]byte, error) {
	buff, err := walk.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	walk.nwqid, buff = FromLittleE16(buff)
	walk.wqid = make([]Qid, walk.nwqid)
	var i uint16
	for ; i < walk.nwqid; i++ {
		buff, err = walk.wqid[i].Parse(buff)
		if err != nil {
			return nil, err
		}
	}
	return buff, nil
}

type TOpen struct {
	FCall
	fid uint32
	mode uint8
}

func (open *TOpen) String() string {
	return fmt.Sprintf("topen: [%s, fid: %d, mode: %d]",
		&open.FCall, open.fid, open.mode)
}

func (open *TOpen) Parse(buff []byte) ([]byte, error) {
	buff, err := open.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	open.fid, buff = FromLittleE32(buff)
	open.mode = buff[0]
	return buff[1:], nil
}

type ROpen struct {
	FCall
	qid Qid
	iounit uint32
}

func (open *ROpen) String() string {
	return fmt.Sprintf("ropen: [%s, qid: [%s], iounit: %d]",
		&open.FCall, &open.qid, open.iounit)
}

func (open *ROpen) Parse(buff []byte) ([]byte, error) {
	buff, err := open.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	buff, err = open.qid.Parse(buff)
	if err != nil {
		return nil, err
	}
	open.iounit, buff = FromLittleE32(buff)
	return buff, nil
}

type TCreate struct {
	FCall
	fid uint32
	name string
	perm uint32
	mode uint8
}

func (create *TCreate) String() string {
	return fmt.Sprintf("tcreate: [%s, fid: %d, name: %s, perm: %d, mode: %d]",
		&create.FCall, create.fid, create.name, create.perm, create.mode)
}

func (create *TCreate) Parse(buff []byte) ([]byte, error) {
	buff, err := create.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	create.fid, buff = FromLittleE32(buff)
	create.name, buff = FromString(buff)
	create.perm, buff = FromLittleE32(buff)
	create.mode = buff[0]
	buff = buff[1:]
	return buff, nil
}

type RCreate struct {
	FCall
	qid Qid
	iounit uint32
}

func (create *RCreate) String() string {
	return fmt.Sprintf("rcreate: [%s, qid: [%s], iounit: %d]",
		&create.FCall, &create.qid, create.iounit)
}

func (create *RCreate) Parse(buff []byte) ([]byte, error) {
	buff, err := create.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	buff, err = create.qid.Parse(buff)
	if err != nil {
		return nil, err
	}

	create.iounit, buff = FromLittleE32(buff)
	return buff, nil
}

type TRead struct {
	FCall
	fid uint32
	offset uint64
	count uint32
}

func (read *TRead) String() string {
	return fmt.Sprintf("tread: [%s, fid: %d, offset: %d, count: %d]",
		&read.FCall, read.fid, read.offset, read.count)
}

func (read *TRead) Parse(buff []byte) ([]byte, error) {
	buff, err := read.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}
	read.fid, buff = FromLittleE32(buff)
	read.offset, buff = FromLittleE64(buff)
	read.count, buff = FromLittleE32(buff)
	return buff, nil
}

type RRead struct {
	FCall
	count uint32
	data []byte
}

func (read *RRead) String() string {
	return fmt.Sprintf("rread: [%s, count: %d]", &read.FCall, read.count)
}

func (read *RRead) Parse(buff []byte) ([]byte, error) {

	buff, err := read.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}
	read.count, buff = FromLittleE32(buff)
	read.data = buff[:read.count - 1]
	return buff[read.count:], nil
}

type TWrite struct {
	FCall
	fid uint32
	offset uint64
	count uint32
	data []byte
}

func (write *TWrite) String() string {
	return fmt.Sprintf("twrite: [%s, fid: %d, offset: %d, count: %d]",
		&write.FCall, write.fid, write.offset, write.count)
}

func (write *TWrite) Parse(buff []byte) ([]byte, error) {
	buff, err := write.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}
	write.fid, buff = FromLittleE32(buff)
	write.offset, buff = FromLittleE64(buff)
	write.count, buff = FromLittleE32(buff)
	write.data = buff[:write.count]
	return buff[write.count:], nil
}

type RWrite struct {
	FCall
	count uint32
}

func (write *RWrite) String() string {
	return fmt.Sprintf("rwrite: [%s, count: %d]", &write.FCall, write.count)
}

func (write *RWrite) Parse(buff []byte) ([]byte, error) {
	buff, err := write.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}
	write.count, buff = FromLittleE32(buff)
	return buff, nil
}

type TClunk struct {
	FCall
	fid uint32
}

func (clunk *TClunk) String() string {
	return fmt.Sprintf("tclunk: [%s, fid: %d]", &clunk.FCall, clunk.fid)
}

func (clunk *TClunk) Parse(buff []byte) ([]byte, error) {
	buff, err := clunk.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	clunk.fid, buff = FromLittleE32(buff)
	return buff, nil
}

type RClunk struct {
	FCall
}

func (clunk *RClunk) String() string {
	return fmt.Sprintf("rclunk: [%s]", &clunk.FCall)
}

func (clunk *RClunk) Parse(buff []byte) ([]byte, error) {
	buff, err := clunk.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	return buff, nil
}

type TRemove struct {
	FCall
	fid uint32
}

func (remove *TRemove) String() string {
	return fmt.Sprintf("tremove: [%s, fid: %d]", &remove.FCall, remove.fid)
}

func (remove *TRemove) Parse(buff []byte) ([]byte, error) {
	buff, err := remove.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	remove.fid, buff = FromLittleE32(buff)
	return buff, nil
}

type RRemove struct {
	FCall
}

func (remove *RRemove) String() string {
	return fmt.Sprintf("rremove: [%s]", &remove.FCall)
}

func (remove *RRemove) Parse(buff []byte) ([]byte, error) {
	buff, err := remove.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	return buff, nil
}

type TStat struct {
	FCall
	fid uint32
}

func (stat *TStat) String() string {
	return fmt.Sprintf("tstat: [%s, fid: %d]", &stat.FCall, stat.fid)
}

func (stat *TStat) Parse(buff []byte) ([]byte, error) {
	buff, err := stat.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	stat.fid, buff = FromLittleE32(buff)
	return buff, nil
}

type Stat struct {
	stype uint16
	dev uint32
	qid Qid
	mode uint32
	atime uint32
	mtime uint32
	length uint64
	name string
	uid string
	gid string
	muid string
}

func (stat *Stat) String() string {
	return fmt.Sprintf("stype: %d, dev: %d, qid: [%s], mode: %d, atime: %d, mtime: %d, length: %d, name: %s, uid: %s, gid: %s, muid: %s",
		stat.stype, stat.dev, &stat.qid, stat.mode,
		stat.atime, stat.mtime, stat.length, stat.name, stat.uid,
		stat.gid, stat.muid)
}

func (stat *Stat) Parse(buff []byte) ([]byte, error) {
	_, buff = FromLittleE16(buff) // throw away length
	stat.stype, buff = FromLittleE16(buff)
	stat.dev, buff = FromLittleE32(buff)
	buff, err := stat.qid.Parse(buff)
	if err != nil {
		return nil, err
	}
	stat.mode, buff = FromLittleE32(buff)
	stat.atime, buff = FromLittleE32(buff)
	stat.mtime, buff = FromLittleE32(buff)
	stat.length, buff = FromLittleE64(buff)
	stat.name, buff = FromString(buff)
	stat.uid, buff = FromString(buff)
	stat.gid, buff = FromString(buff)
	stat.muid, buff = FromString(buff)
	return buff, nil
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
	buff, err := stat.FCall.Parse(buff)
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

type TWstat struct {
	FCall
	fid uint32
	stat Stat
}

func (wstat *TWstat) String() string {
	return fmt.Sprintf("twstat: [%s, fid: %d, %s]",
		&wstat.FCall, wstat.fid, &wstat.stat)
}

func (wstat *TWstat) Parse(buff []byte) ([]byte, error) {
	buff, err := wstat.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}
	wstat.fid, buff = FromLittleE32(buff)
	buff, err = wstat.stat.Parse(buff)
	if err != nil {
		return nil, err
	}
	return buff, nil
}

type RWstat struct {
	FCall
}

func (wstat *RWstat) String() string {
	return fmt.Sprintf("rwstat: [%s]", &wstat.FCall)
}

func (wstat *RWstat) Parse(buff []byte) ([]byte, error) {
	buff, err := wstat.FCall.Parse(buff)
	if err != nil {
		return nil, err
	}

	return buff, nil
}

func readBytes(r io.Reader, buff []byte) error {
	var read int
	var err error

	for read < len(buff) {
		currRead := 0
		currRead, err = r.Read(buff[read:])
		if err != nil {
			return err
		}
		read += currRead
	}
	return nil
}

func FromLittleE16(buff []byte) (uint16, []byte) {
	if len(buff) < 2 {
		return 0, nil
	}
	var ret uint16
	ret = uint16(buff[0]) |
		((uint16(buff[1]) <<  8) & 0x0000FF00)
	return ret, buff[2:]
}

func FromLittleE32(buff []byte) (uint32, []byte) {
	if len(buff) < 4 {
		return 0, nil
	}
	var ret uint32
	ret = uint32(buff[0]) |
		((uint32(buff[1]) <<  8) & 0x0000FF00) |
		((uint32(buff[2]) << 16) & 0x00FF0000) |
		((uint32(buff[3]) << 24) & 0xFF000000);
	return ret, buff[4:]
}

func FromLittleE64(buff []byte) (uint64, []byte) {
	if len(buff) < 8 {
		return 0, nil
	}
	var ret uint64
	ret =
		uint64(buff[0]) |
		((uint64(buff[1]) <<  8) & 0x000000000000FF00) |
		((uint64(buff[2]) << 16) & 0x0000000000FF0000) |
		((uint64(buff[3]) << 24) & 0x00000000FF000000) |
		((uint64(buff[4]) << 32) & 0x000000FF00000000) |
		((uint64(buff[5]) << 40) & 0x0000FF0000000000) |
		((uint64(buff[6]) << 48) & 0x00FF000000000000) |
		((uint64(buff[7]) << 56) & 0xFF00000000000000);
	return ret, buff[8:]
}

func FromString(buff []byte) (string, []byte) {
	var len uint16
	len, buff = FromLittleE16(buff)

	ret := string(buff[:len])
	return ret, buff[len:]
}

type ParseError struct {
	err string
}

func (pe *ParseError) Error() string {
	return pe.err
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
	length, _ := FromLittleE32(sizebuff)

	// Subtract 4 for uint32 length we read
	buff := make([]byte, length - 4)
	err = readBytes(r, buff)
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
	return call, nil
}
