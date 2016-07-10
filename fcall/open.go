package fcall

import "fmt"

type TOpen struct {
	FCall
	Fid uint32
	Mode uint8
}

func (open *TOpen) String() string {
	return fmt.Sprintf("topen: [%s, fid: %d, mode: %d]",
		&open.FCall, open.Fid, open.Mode)
}

func (open *TOpen) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&open.FCall, buff)
	if err != nil {
		return nil, err
	}

	open.Fid, buff = FromLittleE32(buff)
	open.Mode = buff[0]
	return buff[1:], nil
}

func (open *TOpen) Compose() []byte {
	// size[4] Twrite tag[2] fid[4] mode[1]
	length := 4 + 1 + 2 + 4 + 1
	buff := make([]byte, length)
	buffer := buff

	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = open.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(open.Tag, buffer)
	buffer = ToLittleE32(open.Fid, buffer)
	buffer[0] = open.Mode; buffer = buffer[1:]
	return buff
}

type ROpen struct {
	FCall
	Qid Qid
	Iounit uint32
}

func (open *ROpen) String() string {
	return fmt.Sprintf("ropen: [%s, qid: [%s], iounit: %d]",
		&open.FCall, &open.Qid, open.Iounit)
}

func (open *ROpen) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&open.FCall, buff)
	if err != nil {
		return nil, err
	}

	buff, err = open.Qid.Parse(buff)
	if err != nil {
		return nil, err
	}
	open.Iounit, buff = FromLittleE32(buff)
	return buff, nil
}

func (open *ROpen) Compose() []byte {
	// size[4] Ropen tag[2] qid[13] iounit[4]
	length := 4 + 1 + 2 + 13 + 4
	buff := make([]byte, length)
	buffer := buff

	buffer = ToLittleE32(uint32(length), buffer)
	buffer[0] = open.Ctype; buffer = buffer[1:]
	buffer = ToLittleE16(open.Tag, buffer)
	qidbuff := open.Qid.Compose()
	copy(buffer, qidbuff)
	buffer = buffer[len(qidbuff):]
	buffer = ToLittleE32(open.Iounit, buffer)
	return buff
}
