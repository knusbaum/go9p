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

	clunk.Fid, buff = FromLittleE32(buff)
	return buff, nil
}

func (clunk *TClunk) Compose() []byte {
	// size[4] Tclunk tag[2] fid[4]
	length := 4 + 1 + 2 + 4
	buff := make([]byte, length)
	buffer := buff

	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = clunk.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(clunk.Tag, buffer)
	buffer = ToLittleE32(clunk.Fid, buffer)
	return buff
}

func (clunk *TClunk) Reply(fs *Filesystem, conn *Connection, s *Server) IFCall {
	delete(conn.fids, clunk.Fid)
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

	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = clunk.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(clunk.Tag, buffer)
	return buff
}
