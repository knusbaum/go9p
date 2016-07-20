package go9p

import (
	"fmt"
)

type TRVersion struct {
	FCall
	Msize uint32
	Version string
}

func (version *TRVersion) String() string {
	var c byte
	if version.Ctype == Tversion {
		c = 't'
	} else {
		c = 'r'
	}
	return fmt.Sprintf("%cversion: [%s, msize: %d, version: %s]",
		c, &version.FCall, version.Msize, version.Version)
}

func (version *TRVersion) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&version.FCall, buff)
	if err != nil {
		return nil, err
	}

	version.Msize, buff = FromLittleE32(buff)
	version.Version, buff = FromString(buff)
	return buff, nil
}

func (version *TRVersion) Compose() []byte {
	// size[4] Tversion tag[2] msize[4] version[s]
	length := 4 + 1 + 2 + 4 + (2 + len(version.Version))
	buff := make([]byte, length)
	buffer := buff
	
	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = version.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(version.Tag, buffer)
	buffer = ToLittleE32(version.Msize, buffer)
	buffer = ToString(version.Version, buffer)
	
	return buff
}

func (version *TRVersion) Reply(filesystem *Filesystem, conn *Connection, s *Server) IFCall {
	var reply TRVersion = TRVersion{}
	if version.Ctype == Tversion {
		reply = *version
		reply.Ctype = Rversion
		return &reply
	} else {
		return nil;
	}
}
