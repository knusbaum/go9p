package fcall

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

	remove.Fid, buff = FromLittleE32(buff)
	return buff, nil
}

func (remove *TRemove) Compose() []byte {
	// size[4] Tremove tag[2] fid[4]
	length := 4 + 1 + 2 + 4
	buff := make([]byte, length)
	buffer := buff

	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = remove.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(remove.Tag, buffer)
	buffer = ToLittleE32(remove.Fid, buffer)
	return buff
}

func (remove *TRemove) Reply(fs Filesystem, conn Connection) IFCall {
	file := fs.FileForPath(conn.PathForFid(remove.Fid))
	if file == nil {
		return &RError{FCall{Rerror, remove.Tag}, "No such file."}
	}

	// TODO: check permissions
	// if(!fs.permission(...)) { ... }
	fs.RemoveFile(file)

	return &RRemove{FCall{Rremove, remove.Tag}}
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

	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = remove.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(remove.Tag, buffer)
	return buff
}
