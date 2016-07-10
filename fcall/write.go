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
