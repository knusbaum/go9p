package go9p

import (
	"fmt"
)

type TAuth struct {
	FCall
	Afid  uint32
	Uname string
	Aname string
}

func (auth *TAuth) String() string {
	return fmt.Sprintf("tauth: [%s, afid: %d, uname: %s, aname: %s]",
		&auth.FCall, auth.Afid, auth.Uname, auth.Aname)
}

func (auth *TAuth) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&auth.FCall, buff)
	if err != nil {
		return nil, err
	}

	auth.Afid, buff = fromLittleE32(buff)
	auth.Uname, buff = fromString(buff)
	auth.Aname, buff = fromString(buff)
	return buff, nil
}

func (auth *TAuth) Compose() []byte {
	// size[4] Tauth tag[2] afid[4] uname[s] aname[s]
	var length uint32 = uint32(4 + 1 + 2 + 4 +
		(2 + len(auth.Uname)) + (2 + len(auth.Aname)))
	buff := make([]byte, length)
	buffer := buff

	buffer = toLittleE32(length, buffer)
	buffer[0] = auth.Ctype
	buffer = buffer[1:]
	buffer = toLittleE16(auth.Tag, buffer)
	buffer = toLittleE32(auth.Afid, buffer)
	buffer = toString(auth.Uname, buffer)
	buffer = toString(auth.Aname, buffer)

	return buff
}

func shuttleAuthOut(conn *connection, authfuncOut chan []byte, servOut chan outgoing, afid uint32) {
	var tag uint16
	var totalSize uint64
	for m := range authfuncOut {
		writecall := &TWrite{FCall{Twrite, tag},
			afid, totalSize, uint32(len(m)), m}
		fmt.Printf("Sending TWrite for auth: %s\n", writecall)
		servOut <- outgoing{conn, writecall.Compose()}
		tag++
		totalSize += uint64(len(m))
	}
}

func runAuthFunc(fs *filesystem, conn *connection, s *Server, auth *TAuth) {
	conn.setFidPath(auth.Afid, "!AUTH")
	ctx := &AuthContext{Ctx{conn, fs, &auth.FCall, auth.Afid, nil}}
	conn.iauthch = make(chan []byte)
	conn.oauthch = make(chan []byte)
	conn.Afid = int64(auth.Afid)
	fmt.Println("Launching auth data shuttle.\n")
	go shuttleAuthOut(conn, conn.oauthch, s.outgoing, auth.Afid)
	fmt.Println("Running user auth function.\n")
	s.Auth(ctx, conn.iauthch, conn.oauthch) 
	fmt.Println("User auth returned. Cleaning up.\n")
	// Cleanup
	close(conn.iauthch)
	close(conn.oauthch)
	conn.iauthch = nil
	conn.oauthch = nil
	conn.Afid = -1
}

func (auth *TAuth) Reply(fs *filesystem, conn *connection, s *Server) IFCall {
	if s.Auth != nil {
		// We support authentication
		go runAuthFunc(fs, conn, s, auth)
		// qid type: QTAUTH (0x08)
		qid := fs.allocQid(0x08)
		return &RAuth{FCall{Rauth, auth.Tag}, qid}
	} else {
		reply := RError{}
		reply.Ctype = Rerror
		reply.Tag = auth.Tag
		reply.Ename = "Authentication Not Supported."
		return &reply
	}
}

type RAuth struct {
	FCall
	Aqid Qid
}

func (auth *RAuth) String() string {
	return fmt.Sprintf("rauth: [%s, aqid: [%s]]",
		&auth.FCall, &auth.Aqid)
}

func (auth *RAuth) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&auth.FCall, buff)
	if err != nil {
		return nil, err
	}

	buff, err = auth.Aqid.Parse(buff)
	if err != nil {
		return nil, err
	}
	return buff, nil
}

func (auth *RAuth) Compose() []byte {
	// size[4] Rauth tag[2] aqid[13]
	length := 4 + 1 + 2 + 13
	buff := make([]byte, length)
	buffer := buff

	buffer = toLittleE32(uint32(length), buffer)
	buffer[0] = auth.Ctype
	buffer = buffer[1:]
	buffer = toLittleE16(auth.Tag, buffer)
	qidbuffer := auth.Aqid.Compose()
	copy(buffer, qidbuffer)
	return buff
}
