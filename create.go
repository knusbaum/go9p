package go9p

import (
	"fmt"
	//	"time"
)

type TCreate struct {
	FCall
	Fid  uint32
	Name string
	Perm uint32
	Mode uint8
}

func (create *TCreate) String() string {
	return fmt.Sprintf("tcreate: [%s, fid: %d, name: %s, perm: %o, mode: %d]",
		&create.FCall, create.Fid, create.Name, create.Perm, create.Mode)
}

func (create *TCreate) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&create.FCall, buff)
	if err != nil {
		return nil, err
	}

	create.Fid, buff = fromLittleE32(buff)
	create.Name, buff = fromString(buff)
	create.Perm, buff = fromLittleE32(buff)
	create.Mode = buff[0]
	buff = buff[1:]
	return buff, nil
}

func (create *TCreate) Compose() []byte {
	// size[4] Tcreate tag[2] fid[4] name[s] perm[4] mode[1]
	length := 4 + 1 + 2 + 4 + (2 + len(create.Name)) + 4 + 1
	buff := make([]byte, length)
	buffer := buff

	buffer = toLittleE32(uint32(length), buffer)
	buffer[0] = create.Ctype
	buffer = buffer[1:]
	buffer = toLittleE16(create.Tag, buffer)
	buffer = toLittleE32(create.Fid, buffer)
	buffer = toString(create.Name, buffer)
	buffer = toLittleE32(create.Perm, buffer)
	buffer[0] = create.Mode
	buffer = buffer[1:]
	return buff
}

func (create *TCreate) Reply(fs *filesystem, conn *connection, s *Server) IFCall {
	file := fs.fileForPath(conn.pathForFid(create.Fid))
	if file == nil {
		return &RError{FCall{rerror, create.Tag}, "No such file."}
	}

	if !openPermission(conn.uname, file, Owrite) {
		return &RError{FCall{rerror, create.Tag}, "Permission denied."}
	}

	path := ""
	if file.Path == "/" {
		path = file.Path + create.Name
	} else {
		path = file.Path + "/" + create.Name
	}

	if s.Create != nil {
		ctx := &Createcontext{
			Ctx{conn, fs, &create.FCall, create.Fid, file},
			path,
			create.Name,
			create.Perm,
			create.Mode}
		s.Create(ctx)
	} else {
		return &RError{FCall{rerror, create.Tag}, "Create not implemented."}
	}
	return nil
	//	newfile :=
	//		fs.AddFile(path,
	//		Stat{
	//			Stype: 0,
	//			Dev: 0,
	//			Qid: fs.AllocQid(uint8(create.Perm >> 24)),
	//			Mode: create.Perm,
	//			Atime: uint32(time.Now().Unix()),
	//			Mtime: uint32(time.Now().Unix()),
	//			Length: 0,
	//			Name: create.Name,
	//			Uid: conn.uname,
	//			Gid: file.stat.Gid,
	//			Muid: conn.uname},
	//		file)
	//
	//	conn.SetFidPath(create.Fid, path)
	//	conn.SetFidOpenmode(create.Fid, Ordwr)
	//
	//	fmt.Println("currUid: ", fs.currUid)
	//
	//	return &RCreate{FCall{Rcreate, create.Tag}, newfile.stat.Qid, iounit}
}

type RCreate struct {
	FCall
	Qid    Qid
	Iounit uint32
}

func (create *RCreate) String() string {
	return fmt.Sprintf("rcreate: [%s, qid: [%s], iounit: %d]",
		&create.FCall, &create.Qid, create.Iounit)
}

func (create *RCreate) Parse(buff []byte) ([]byte, error) {
	buff, err := fcParse(&create.FCall, buff)
	if err != nil {
		return nil, err
	}

	buff, err = create.Qid.Parse(buff)
	if err != nil {
		return nil, err
	}

	create.Iounit, buff = fromLittleE32(buff)
	return buff, nil
}

func (create *RCreate) Compose() []byte {
	// size[4] Rcreate tag[2] qid[13] iounit[4]
	length := 4 + 1 + 2 + 13 + 4
	buff := make([]byte, length)
	buffer := buff

	buffer = toLittleE32(uint32(length), buffer)
	buffer[0] = create.Ctype
	buffer = buffer[1:]
	buffer = toLittleE16(create.Tag, buffer)
	qidbuff := create.Qid.Compose()
	copy(buffer, qidbuff)
	buffer = buffer[len(qidbuff):]
	buffer = toLittleE32(create.Iounit, buffer)
	return buff
}
