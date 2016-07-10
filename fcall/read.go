package fcall

import "fmt"

type TRead struct {
	FCall
	Fid uint32
	Offset uint64
	Count uint32
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
	read.Fid, buff = FromLittleE32(buff)
	read.Offset, buff = FromLittleE64(buff)
	read.Count, buff = FromLittleE32(buff)
	return buff, nil
}

func (read *TRead) Compose() []byte {
	// size[4] Twrite tag[2] fid[4] offset[8] count[4]
	length := 4 + 1 + 2 + 4 + 8 + 4
	buff := make([]byte, length)
	buffer := buff

	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = read.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(read.Tag, buffer)
	buffer = ToLittleE32(read.Fid, buffer)
	buffer = ToLittleE64(read.Offset, buffer)
	buffer = ToLittleE32(read.Count, buffer)
	return buff
}

type RRead struct {
	FCall
	Count uint32
	Data []byte
}

func (read *RRead) String() string {
	return fmt.Sprintf("rread: [%s, count: %d]", &read.FCall, read.Count)
}

func (read *RRead) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&read.FCall, buff)
	if err != nil {
		return nil, err
	}
	read.Count, buff = FromLittleE32(buff)
	read.Data = make([]byte, read.Count)
	copy(read.Data, buff[:read.Count])
	return buff[read.Count:], nil
}

func (read *RRead) Compose() []byte {
	// size[4] Rread tag[2] count[4] data[count]
	length := 4 + 1 + 2 + 4 + read.Count
	buff := make([]byte, length)
	buffer := buff

	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = read.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(read.Tag, buffer)
	buffer = ToLittleE32(read.Count, buffer)
	copy(buffer, read.Data)
	return buff
}
