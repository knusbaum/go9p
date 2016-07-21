package go9p

import "fmt"

type TFlush struct {
	FCall
	Oldtag uint16
}

func (flush *TFlush) String() string {
	return fmt.Sprintf("tflush: [%s, oldtag: %d]",
		&flush.FCall, flush.Oldtag)
}

func (flush *TFlush) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&flush.FCall, buff)
	if err != nil {
		return nil, err
	}

	flush.Oldtag, buff = fromLittleE16(buff)
	return buff, nil
}

func (flush *TFlush) Compose() []byte {
	// size[4] Tflush tag[2] oldtag[2]
	length := 4 + 1 + 2 + 2
	buff := make([]byte, length)
	buffer := buff

	buffer = toLittleE32(uint32(length), buffer)
	buffer[0] = flush.Ctype
	buffer = buffer[1:]
	buffer = toLittleE16(flush.Tag, buffer)
	buffer = toLittleE16(flush.Oldtag, buffer)
	return buff
}

type RFlush struct {
	FCall
}

func (flush *RFlush) String() string {
	return fmt.Sprintf("rflush: [%s]", &flush.FCall)
}

func (flush *RFlush) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&flush.FCall, buff)
	if err != nil {
		return nil, err
	}

	return buff, nil
}

func (flush *RFlush) Compose() []byte {
	// size[4] Rflush tag[2]
	length := 4 + 1 + 2
	buff := make([]byte, length)
	buffer := buff

	buffer = toLittleE32(uint32(length), buffer)
	buffer[0] = flush.Ctype
	buffer = buffer[1:]
	buffer = toLittleE16(flush.Tag, buffer)
	return buff
}
