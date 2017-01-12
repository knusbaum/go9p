package go9p

import "fmt"

type TRemove struct {
	FCall
	Fid uint32
}

func (remove *TRemove) String() string {
	return fmt.Sprintf("tremove: [%s, fid: %d]", &remove.FCall, remove.Fid)
}

func (remove *TRemove) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&remove.FCall, buff)
	if err != nil {
		return nil, err
	}

	remove.Fid, buff = fromLittleE32(buff)
	return buff, nil
}

func (remove *TRemove) Compose() []byte {
	// size[4] Tremove tag[2] fid[4]
	length := 4 + 1 + 2 + 4
	buff := make([]byte, length)
	buffer := buff

	buffer = toLittleE32(uint32(length), buffer)
	buffer[0] = remove.Ctype
	buffer = buffer[1:]
	buffer = toLittleE16(remove.Tag, buffer)
	buffer = toLittleE32(remove.Fid, buffer)
	return buff
}

func (remove *TRemove) Reply(fs *filesystem, conn *connection, s *Server) IFCall {
	file := fs.fileForPath(conn.pathForFid(remove.Fid))
	if file == nil {
		return &RError{FCall{Rerror, remove.Tag}, "No such file."}
	}
	if !openPermission(conn.uname, file, Owrite) {
		return &RError{FCall{Rerror, remove.Tag}, "Permission denied."}
	}

	if s.Remove != nil {
		ctx := &RemoveContext{Ctx{conn, fs, &remove.FCall, remove.Fid, file}}
		s.Remove(ctx)
	} else {
		return &RError{FCall{Rerror, remove.Tag}, "Remove not implemented."}
	}

	return nil
}

type RRemove struct {
	FCall
}

func (remove *RRemove) String() string {
	return fmt.Sprintf("rremove: [%s]", &remove.FCall)
}

func (remove *RRemove) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&remove.FCall, buff)
	if err != nil {
		return nil, err
	}

	return buff, nil
}

func (remove *RRemove) Compose() []byte {
	// size[4] Rwstat tag[2]
	length := 4 + 1 + 2
	buff := make([]byte, length)
	buffer := buff

	buffer = toLittleE32(uint32(length), buffer)
	buffer[0] = remove.Ctype
	buffer = buffer[1:]
	buffer = toLittleE16(remove.Tag, buffer)
	return buff
}
