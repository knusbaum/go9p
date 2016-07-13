package fcall

import (
	"fmt"
)

type RError struct {
	FCall
	Ename string
}

func (error *RError) String() string {
	return fmt.Sprintf("rerror: [%s, ename: %s]",
		&error.FCall, error.Ename)
}

func (error *RError) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&error.FCall, buff)
	if err != nil {
		return nil, err
	}

	error.Ename, buff = FromString(buff)
	return buff, nil
}

func (error *RError) Compose() []byte {
	// size[4] Rerror tag[2] ename[s]
	length := 4 + 1 + 2 + (2 + len(error.Ename))
	buff := make([]byte, length)
	buffer := buff

	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = error.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(error.Tag, buffer)
	buffer = ToString(error.Ename, buffer)

	return buff
}

func (error *RError) Reply (filesystem Filesystem, conn Connection) IFCall {
	return nil
}
