package go9p

import "fmt"

type TClunk struct {
	FCall
	Fid uint32
}

func (clunk *TClunk) String() string {
	return fmt.Sprintf("tclunk: [%s, fid: %d]", &clunk.FCall, clunk.Fid)
}

func (clunk *TClunk) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&clunk.FCall, buff)
	if err != nil {
		return nil, err
	}

	clunk.Fid, buff = fromLittleE32(buff)
	return buff, nil
}

func (clunk *TClunk) Compose() []byte {
	// size[4] Tclunk tag[2] fid[4]
	length := 4 + 1 + 2 + 4
	buff := make([]byte, length)
	buffer := buff

	buffer = toLittleE32(uint32(length), buffer)
	buffer[0] = clunk.Ctype
	buffer = buffer[1:]
	buffer = toLittleE16(clunk.Tag, buffer)
	buffer = toLittleE32(clunk.Fid, buffer)
	return buff
}

func (clunk *TClunk) Reply(fs *filesystem, conn *connection, s *Server) IFCall {
	file := fs.fileForPath(conn.pathForFid(clunk.Fid))
	openmode := conn.getFidOpenmode(clunk.Fid)

	// This should be turned into a function on conn.
	// We shouldn't be mucking around in conn's members.
	delete(conn.fids, clunk.Fid)
	delete(conn.dirContents, clunk.Fid)
	conn.getReadCalled()[clunk.Fid] = false
	
	if openmode != None &&
		s.Close != nil {
		ctx := &Ctx{conn, fs, &clunk.FCall, clunk.Fid, file}
		s.Close(ctx)
	}
	return &RClunk{FCall{Rclunk, clunk.Tag}}
}

type RClunk struct {
	FCall
}

func (clunk *RClunk) String() string {
	return fmt.Sprintf("rclunk: [%s]", &clunk.FCall)
}

func (clunk *RClunk) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&clunk.FCall, buff)
	if err != nil {
		return nil, err
	}

	return buff, nil
}

func (clunk *RClunk) Compose() []byte {
	// size[4] Rclunk tag[2]
	length := 4 + 1 + 2
	buff := make([]byte, length)
	buffer := buff

	buffer = toLittleE32(uint32(length), buffer)
	buffer[0] = clunk.Ctype
	buffer = buffer[1:]
	buffer = toLittleE16(clunk.Tag, buffer)
	return buff
}
