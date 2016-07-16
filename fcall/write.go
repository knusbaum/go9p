package fcall

import "fmt"

type TWrite struct {
	FCall
	Fid uint32
	Offset uint64
	Count uint32
	Data []byte
}

func (write *TWrite) String() string {
	return fmt.Sprintf("twrite: [%s, fid: %d, offset: %d, count: %d]",
		&write.FCall, write.Fid, write.Offset, write.Count)
}

func (write *TWrite) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&write.FCall, buff)
	if err != nil {
		return nil, err
	}
	write.Fid, buff = FromLittleE32(buff)
	write.Offset, buff = FromLittleE64(buff)
	write.Count, buff = FromLittleE32(buff)
	write.Data = make([]byte, write.Count)
	copy(write.Data, buff[:write.Count])
	return buff[write.Count:], nil
}

func (write *TWrite) Compose() []byte {
	// size[4] Twrite tag[2] fid[4] offset[8] count[4] data[count]
	length := 4 + 1 + 2 + 4 + 8 + 4 + write.Count
	buff := make([]byte, length)
	buffer := buff

	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = write.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(write.Tag, buffer)
	buffer = ToLittleE32(write.Fid, buffer)
	buffer = ToLittleE64(write.Offset, buffer)
	buffer = ToLittleE32(write.Count, buffer)
	copy(buffer, write.Data)
	return buff
}

func (write *TWrite) Reply(fs *Filesystem, conn *Connection) IFCall {
	file := fs.FileForPath(conn.PathForFid(write.Fid))
	if file == nil {
		return &RError{FCall{Rerror, write.Tag}, "No such file."}
	}
	openmode := conn.GetFidOpenmode(write.Fid)

	if (openmode & 0x0F) != Owrite &&
		(openmode & 0x0F) != Ordwr {
		return &RError{FCall{Rerror, write.Tag}, "File notopened for write."}
	} else if (file.stat.Mode & (1<<31)) != 0 {
		return &RError{FCall{Rerror, write.Tag}, "Cannot write to directory."}
	}

	offset := write.Offset
	if openmode & 0x10 == 0 {
		// If we're not truncating, 0 offset is from EOF.
		foffset := conn.GetFidOpenoffset(write.Fid)
		offset += foffset
	}

	if offset + uint64(write.Count) > file.stat.Length {
		// Extending the file.
		newlen := offset + uint64(write.Count)
		file.stat.Length = newlen
		newbuff := make([]byte, newlen - uint64(len(file.Contents)) )
		file.Contents = append(file.Contents, newbuff...)
	}

	copy(file.Contents[offset:offset+uint64(write.Count)], write.Data)
	return &RWrite{FCall{Rwrite, write.Tag}, write.Count}
}

type RWrite struct {
	FCall
	Count uint32
}

func (write *RWrite) String() string {
	return fmt.Sprintf("rwrite: [%s, count: %d]", &write.FCall, write.Count)
}

func (write *RWrite) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&write.FCall, buff)
	if err != nil {
		return nil, err
	}
	write.Count, buff = FromLittleE32(buff)
	return buff, nil
}

func (write *RWrite) Compose() []byte {
	// size[4] Rwrite tag[2] count[4]
	length := 4 + 1 + 2 + 4
	buff := make([]byte, length)
	buffer := buff

	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = write.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(write.Tag, buffer)
	buffer = ToLittleE32(write.Count, buffer)
	return buff
}
