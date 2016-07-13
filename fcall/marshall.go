package fcall

import "io"

func ReadBytes(r io.Reader, buff []byte) error {
	var read int
	var err error

	for read < len(buff) {
		currRead := 0
		currRead, err = r.Read(buff[read:])
		if err != nil {
			return err
		}
		read += currRead
	}
	return nil
}

func FromLittleE16(buff []byte) (uint16, []byte) {
	if len(buff) < 2 {
		return 0, nil
	}
	var ret uint16
	ret = uint16(buff[0]) |
		((uint16(buff[1]) <<  8) & 0x0000FF00)
	return ret, buff[2:]
}

func FromLittleE32(buff []byte) (uint32, []byte) {
	if len(buff) < 4 {
		return 0, nil
	}
	var ret uint32
	ret = uint32(buff[0]) |
		((uint32(buff[1]) <<  8) & 0x0000FF00) |
		((uint32(buff[2]) << 16) & 0x00FF0000) |
		((uint32(buff[3]) << 24) & 0xFF000000);
	return ret, buff[4:]
}

func FromLittleE64(buff []byte) (uint64, []byte) {
	if len(buff) < 8 {
		return 0, nil
	}
	var ret uint64
	ret =
		uint64(buff[0]) |
		((uint64(buff[1]) <<  8) & 0x000000000000FF00) |
		((uint64(buff[2]) << 16) & 0x0000000000FF0000) |
		((uint64(buff[3]) << 24) & 0x00000000FF000000) |
		((uint64(buff[4]) << 32) & 0x000000FF00000000) |
		((uint64(buff[5]) << 40) & 0x0000FF0000000000) |
		((uint64(buff[6]) << 48) & 0x00FF000000000000) |
		((uint64(buff[7]) << 56) & 0xFF00000000000000);
	return ret, buff[8:]
}

func FromString(buff []byte) (string, []byte) {
	var len uint16
	len, buff = FromLittleE16(buff)

	ret := string(buff[:len])
	return ret, buff[len:]
}

func ToLittleE16(i uint16, buff []byte) []byte {
	buff[0] = byte(i & 0xFF)
	buff[1] = byte((i >> 8) & 0xFF)
	return buff[2:]
}

func ToLittleE32(i uint32, buff []byte) []byte {
	buff[0] = byte(i & 0xFF)
	buff[1] = byte((i >> 8) & 0xFF)
	buff[2] = byte((i >> 16) & 0xFF)
	buff[3] = byte((i >> 24) & 0xFF)
	return buff[4:]
}

func ToLittleE64(i uint64, buff []byte) []byte {
	buff[0] = byte(i & 0xFF)
	buff[1] = byte((i >> 8) & 0xFF)
	buff[2] = byte((i >> 16) & 0xFF)
	buff[3] = byte((i >> 24) & 0xFF)
	
	buff[4] = byte((i >> 32) & 0xFF)
	buff[5] = byte((i >> 40) & 0xFF)
	buff[6] = byte((i >> 48) & 0xFF)
	buff[7] = byte((i >> 56) & 0xFF)

	return buff[8:]
}

func ToString(s string, buff []byte) []byte {
	buff = ToLittleE16(uint16(len(s)), buff)
	copy(buff, []byte(s))
	return buff[len(s):]
}

type ParseError struct {
	Err string
}

func (pe *ParseError) Error() string {
	return pe.Err
}
