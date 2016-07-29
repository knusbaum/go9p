package go9p

import (
	"fmt"
)

const (
	iounit = 8168
)

type TOpen struct {
	FCall
	Fid  uint32
	Mode uint8
}

func (open *TOpen) String() string {
	return fmt.Sprintf("topen: [%s, fid: %d, mode: %d]",
		&open.FCall, open.Fid, open.Mode)
}

func (open *TOpen) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&open.FCall, buff)
	if err != nil {
		return nil, err
	}

	open.Fid, buff = fromLittleE32(buff)
	open.Mode = buff[0]
	return buff[1:], nil
}

func (open *TOpen) Compose() []byte {
	// size[4] Twrite tag[2] fid[4] mode[1]
	length := 4 + 1 + 2 + 4 + 1
	buff := make([]byte, length)
	buffer := buff

	buffer = toLittleE32(uint32(length), buffer)
	buffer[0] = open.Ctype
	buffer = buffer[1:]
	buffer = toLittleE16(open.Tag, buffer)
	buffer = toLittleE32(open.Fid, buffer)
	buffer[0] = open.Mode
	buffer = buffer[1:]
	return buff
}

func (open *TOpen) Reply(fs *filesystem, conn *connection, s *Server) IFCall {
	file := fs.fileForPath(conn.pathForFid(open.Fid))
	if file == nil {
		return &RError{FCall{rerror, open.Tag}, "No such file."}
	}

	openmode := conn.getFidOpenmode(open.Fid)

	if openmode != None {
		return &RError{FCall{rerror, open.Tag}, "Fid already open."}
	}
	if !openPermission(conn.uname, file, open.Mode&0x0F) {
		return &RError{FCall{rerror, open.Tag}, "Permission denied."}
	}

	if file.Stat.Mode&(1<<31) != 0 {
		// This is a directory.
		if (open.Mode&0x0F) == Owrite ||
			(open.Mode&0x0F) == Ordwr {
			return &RError{FCall{rerror, open.Tag}, "Cannot write to directory."}
		}
	}

	if s.Open != nil {
		ctx := &OpenContext{Ctx{conn, fs, &open.FCall, open.Fid, file}, open.Mode}
		s.Open(ctx)
	} else {
		conn.setFidOpenmode(open.Fid, open.Mode)
		conn.setFidOpenoffset(open.Fid, file.Stat.Length)
		if file.Stat.Mode & (1 << 31) != 0 {
			// If this is a directory, write out all subfile stats now so we have a consistent
			// view of the directory throughout the life of the Fid
			conn.setDirContents(open.Fid, file.composeSubfiles())
		}
		return &ROpen{FCall{ropen, open.Tag}, file.Stat.Qid, iounit}
	}
	return nil
}

type ROpen struct {
	FCall
	Qid    Qid
	Iounit uint32
}

func (open *ROpen) String() string {
	return fmt.Sprintf("ropen: [%s, qid: [%s], iounit: %d]",
		&open.FCall, &open.Qid, open.Iounit)
}

func (open *ROpen) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&open.FCall, buff)
	if err != nil {
		return nil, err
	}

	buff, err = open.Qid.Parse(buff)
	if err != nil {
		return nil, err
	}
	open.Iounit, buff = fromLittleE32(buff)
	return buff, nil
}

func (open *ROpen) Compose() []byte {
	// size[4] Ropen tag[2] qid[13] iounit[4]
	length := 4 + 1 + 2 + 13 + 4
	buff := make([]byte, length)
	buffer := buff

	buffer = toLittleE32(uint32(length), buffer)
	buffer[0] = open.Ctype
	buffer = buffer[1:]
	buffer = toLittleE16(open.Tag, buffer)
	qidbuff := open.Qid.Compose()
	copy(buffer, qidbuff)
	buffer = buffer[len(qidbuff):]
	buffer = toLittleE32(open.Iounit, buffer)
	return buff
}
