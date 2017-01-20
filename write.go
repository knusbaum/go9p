package go9p

import "fmt"

type TWrite struct {
	FCall
	Fid    uint32
	Offset uint64
	Count  uint32
	Data   []byte
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
	write.Fid, buff = fromLittleE32(buff)
	write.Offset, buff = fromLittleE64(buff)
	write.Count, buff = fromLittleE32(buff)
	write.Data = make([]byte, write.Count)
	copy(write.Data, buff[:write.Count])
	return buff[write.Count:], nil
}

func (write *TWrite) Compose() []byte {
	// size[4] Twrite tag[2] fid[4] offset[8] count[4] data[count]
	length := 4 + 1 + 2 + 4 + 8 + 4 + write.Count
	buff := make([]byte, length)
	buffer := buff

	buffer = toLittleE32(uint32(length), buffer)
	buffer[0] = write.Ctype
	buffer = buffer[1:]
	buffer = toLittleE16(write.Tag, buffer)
	buffer = toLittleE32(write.Fid, buffer)
	buffer = toLittleE64(write.Offset, buffer)
	buffer = toLittleE32(write.Count, buffer)
	copy(buffer, write.Data)
	return buff
}

func (write *TWrite) Reply(fs *filesystem, conn *connection, s *Server) IFCall {
	file := fs.fileForPath(conn.pathForFid(write.Fid))
	if file == nil {
		if int64(write.Fid) == conn.Afid {
			// The client is writing to the auth FID.
			// Just pull the data out and send it on the auth channel.
			fmt.Println("This is an auth write. Delivering to auth method.")
			conn.iauthch <- write.Data
			return &RWrite{FCall{Rwrite, write.Tag}, write.Count}
		} else {
			return &RError{FCall{Rerror, write.Tag}, "No such file."}
		}
	}
	openmode := conn.getFidOpenmode(write.Fid)

	if (openmode&0x0F) != Owrite &&
		(openmode&0x0F) != Ordwr {
		return &RError{FCall{Rerror, write.Tag}, "File notopened for write."}
	} else if (file.Stat.Mode & (1 << 31)) != 0 {
		return &RError{FCall{Rerror, write.Tag}, "Cannot write to directory."}
	}

	offset := write.Offset
	if openmode&0x10 == 0 {
		// If we're not truncating, 0 offset is from EOF.
		foffset := conn.getFidOpenoffset(write.Fid)
		offset += foffset
	}

	if s.Write != nil {
		ctx := &WriteContext{
			Ctx{conn, fs, &write.FCall, write.Fid, file},
			write.Data,
			offset,
			write.Count}
		s.Write(ctx)
	} else {
		return &RError{FCall{Rerror, write.Tag}, "Write not implemented."}
	}
	return nil
	//	offset := write.Offset
	//	if openmode & 0x10 == 0 {
	//		// If we're not truncating, 0 offset is from EOF.
	//		foffset := conn.GetFidOpenoffset(write.Fid)
	//		offset += foffset
	//	}
	//
	//	if offset + uint64(write.Count) > file.stat.Length {
	//		// Extending the file.
	//		newlen := offset + uint64(write.Count)
	//		file.stat.Length = newlen
	//		newbuff := make([]byte, newlen - uint64(len(file.Contents)) )
	//		file.Contents = append(file.Contents, newbuff...)
	//	}
	//
	//	copy(file.Contents[offset:offset+uint64(write.Count)], write.Data)
	//	return &RWrite{FCall{Rwrite, write.Tag}, write.Count}
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
	write.Count, buff = fromLittleE32(buff)
	return buff, nil
}

func (write *RWrite) Compose() []byte {
	// size[4] Rwrite tag[2] count[4]
	length := 4 + 1 + 2 + 4
	buff := make([]byte, length)
	buffer := buff

	buffer = toLittleE32(uint32(length), buffer)
	buffer[0] = write.Ctype
	buffer = buffer[1:]
	buffer = toLittleE16(write.Tag, buffer)
	buffer = toLittleE32(write.Count, buffer)
	return buff
}
