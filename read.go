package go9p

import "fmt"

type TRead struct {
	FCall
	Fid    uint32
	Offset uint64
	Count  uint32
}

func (read *TRead) String() string {
	return fmt.Sprintf("tread: [%s, fid: %d, offset: %d, count: %d]",
		&read.FCall, read.Fid, read.Offset, read.Count)
}

func (read *TRead) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&read.FCall, buff)
	if err != nil {
		return nil, err
	}
	read.Fid, buff = fromLittleE32(buff)
	read.Offset, buff = fromLittleE64(buff)
	read.Count, buff = fromLittleE32(buff)
	return buff, nil
}

func (read *TRead) Compose() []byte {
	// size[4] Twrite tag[2] fid[4] offset[8] count[4]
	length := 4 + 1 + 2 + 4 + 8 + 4
	buff := make([]byte, length)
	buffer := buff

	buffer = toLittleE32(uint32(length), buffer)
	buffer[0] = read.Ctype
	buffer = buffer[1:]
	buffer = toLittleE16(read.Tag, buffer)
	buffer = toLittleE32(read.Fid, buffer)
	buffer = toLittleE64(read.Offset, buffer)
	buffer = toLittleE32(read.Count, buffer)
	return buff
}

func doDirRead(read *TRead, file *File, conn *connection) *RRead{
	
	contents := make([]byte, 0)
	files := file.subfiles
	fmt.Printf("Have %d subfiles.\n", len(files))
	var length uint64 = 0
	startOffset := -1
	//i := 0
	for i := range files {
		nextLength := uint64(files[i].Stat.ComposeLength())
		if length + nextLength > read.Offset {
			startOffset = i
			break
		}
		length = length + nextLength
	}

	fmt.Println("Start Offset:", startOffset)
	
	if startOffset < 0 {
		return &RRead{FCall{rread, read.Tag}, 0, nil}
	}

	for _, f := range files[startOffset:] {
		nextLength := uint32(f.Stat.ComposeLength())
		if uint32(len(contents)) + nextLength > read.Count {
			break
		}
		contents = append(contents, f.Stat.Compose()...)
	}
	
//	var count uint64 = 0
//	if read.Offset+uint64(read.Count) > uint64(len(contents)) {
//		count = uint64(len(contents)) - read.Offset
//	} else {
//		count = uint64(read.Count)
//	}
//
//	fmt.Println("Contents len:", len(contents))
//	fmt.Println("Offset:", read.Offset, "Count:", count)
//	data := make([]byte, count)
//	if count > 0 {
//		fmt.Println("Contents len:", len(contents))
//		fmt.Println("Offset:", read.Offset, "Count:", count)
//		fmt.Println("Data len:", len(data))
//		copy(data, contents[read.Offset:read.Offset+count])
//	}
	
	return &RRead{FCall{rread, read.Tag}, uint32(len(contents)), contents}
}

func (read *TRead) Reply(fs *filesystem, conn *connection, s *Server) IFCall {
	if read.Count > iounit {
		return &RError{FCall{rerror, read.Tag}, "Read size too large."}
	}

	file := fs.fileForPath(conn.pathForFid(read.Fid))
	if file == nil {
		return &RError{FCall{rerror, read.Tag}, "Failed to read from FID."}
	}

	openmode := conn.getFidOpenmode(read.Fid)
	if (openmode&0x0F) != Oread &&
		(openmode&0x0F) != Ordwr &&
		(openmode&0x0F) != Oexec {
		return &RError{FCall{rerror, read.Tag}, "File not opened."}
	}

	if file.Stat.Mode&(1<<31) != 0 {
		// This is a directory.
		if s.DirRead != nil && conn.readCalled[read.Fid] == false {
			// If the user has hooked the DirRead event, run that. (only on first read message)
			ctx := &DirReadContext{Ctx{conn, fs, &read.FCall, read.Fid, file}, read}
			// DirReadContext.Respond() will call doDirRead.
			s.DirRead(ctx)
			conn.getReadCalled()[read.Fid] = true
			return nil
		} else {
			// Otherwise return the RRead right away.
			conn.getReadCalled()[read.Fid] = true
			return doDirRead(read, file, conn)
		}
		
	} else {
		count := uint64(read.Count)

		if s.Read != nil {
			ctx := &ReadContext{
				Ctx{conn, fs, &read.FCall, read.Fid, file},
				read.Offset,
				uint32(count)}
			s.Read(ctx)
			conn.getReadCalled()[read.Fid] = true
			return nil
		} else {
			conn.getReadCalled()[read.Fid] = true
			return &RError{FCall{rerror, read.Tag}, "Read not implemented."}
		}
	}
}

type RRead struct {
	FCall
	Count uint32
	Data  []byte
}

func (read *RRead) String() string {
	return fmt.Sprintf("rread: [%s, count: %d]", &read.FCall, read.Count)
}

func (read *RRead) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&read.FCall, buff)
	if err != nil {
		return nil, err
	}
	read.Count, buff = fromLittleE32(buff)
	read.Data = make([]byte, read.Count)
	copy(read.Data, buff[:read.Count])
	return buff[read.Count:], nil
}

func (read *RRead) Compose() []byte {
	// size[4] Rread tag[2] count[4] data[count]
	length := 4 + 1 + 2 + 4 + read.Count
	buff := make([]byte, length)
	buffer := buff

	buffer = toLittleE32(uint32(length), buffer)
	buffer[0] = read.Ctype
	buffer = buffer[1:]
	buffer = toLittleE16(read.Tag, buffer)
	buffer = toLittleE32(read.Count, buffer)
	copy(buffer, read.Data)
	return buff
}
