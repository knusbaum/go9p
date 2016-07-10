package fcall

import "fmt"

type TRVersion struct {
	FCall
	Msize uint32
	Version string
}

func (version *TRVersion) String() string {
	return fmt.Sprintf("(t|r)version: [%s, msize: %d, version: %s]",
		&version.FCall, version.Msize, version.Version)
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
