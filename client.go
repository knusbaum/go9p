package go9p

import (
	"io"
	"strings"
	"errors"
	"fmt"
)

type Client struct {
	tags []uint16
	tagmax uint16
	fidmax uint32
	root uint32
	conn io.ReadWriter
	msize uint32
}

type File9P struct {
	fid uint32
	offset uint64
	cli *Client
	iounit uint32
}

func (cli *Client) Connect(rw io.ReadWriter) error {

	version := &TRVersion{FCall{Tversion, ^uint16(0)}, 4096, "9P2000"}
	fmt.Printf(">>> %s\n", version)
	rw.Write(version.Compose())
	ifcall, err := ParseCall(rw)
	fmt.Printf("<<< %s\n", ifcall)
	if err != nil {
		return err
	}
	if ifcall.GetFCall().Ctype != Rversion {
		if ifcall.GetFCall().Ctype == Rerror {
			message := ifcall.(*RError).Ename
			return errors.New(message)
		}
		return errors.New("Unknown Server Protocol Error.")
	}

	msize := ifcall.(*TRVersion).Msize

	attach := &TAttach{FCall{Tattach, 1}, 1, ^uint32(0), "knusbaum", ""}
	fmt.Printf(">>> %s\n", attach)
	rw.Write(attach.Compose())
	ifcall, err = ParseCall(rw)
	fmt.Printf("<<< %s\n", ifcall)
	if err != nil {
		return err
	}
	if ifcall.GetFCall().Ctype != Rattach {
		if ifcall.GetFCall().Ctype == Rerror {
			message := ifcall.(*RError).Ename
			return errors.New(message)
		}
		return errors.New("Unknown Server Protocol Error.")
	}

	cli.conn = rw
	cli.root = 1
	cli.fidmax = 2
	cli.tagmax = 1
	cli.msize = msize
	return nil
}

func (cli *Client) allocTag() uint16 {
	// For now, this is single-threaded and only supports 1 rpc call at a time,
	// so the tag can always be the same.
	return 1
}

func (cli *Client) freeTag(tag uint16) {
}

func (cli *Client) allocFid() uint32 {
	cli.fidmax++
	return cli.fidmax
}

func (cli *Client) freeFid(fid uint32) {
}

func (cli *Client) walk(path string) (uint32, error) {
	var wname []string
	if path == "" {
		wname = make([]string, 0)
	} else {
		wname = strings.Split(path, "/")
		fmt.Printf("path: [%s], wname: len: %d, [%s]\n", path, len(wname), wname)
	}
	newFid := cli.allocFid()
	newTag := cli.allocTag()
	defer cli.freeTag(newTag)
	walk := &TWalk{FCall{Twalk, cli.allocTag()}, cli.root, newFid, uint16(len(wname)), wname}
	fmt.Printf(">>> %s\n", walk)
	cli.conn.Write(walk.Compose())
	ifcall, err := ParseCall(cli.conn)
	fmt.Printf("<<< %s\n", ifcall)
	if err != nil {
		return 0, err
	}
	if ifcall.GetFCall().Ctype != Rwalk {
		if ifcall.GetFCall().Ctype == Rerror {
			message := ifcall.(*RError).Ename
			return 0, errors.New(message)
		}
		return 0, errors.New("Unknown Server Protocol Error.")
	}

	return newFid, nil
}

func (cli *Client) clunk(fid uint32) error {
	tag := cli.allocTag()
	defer cli.freeTag(tag)
	
	clunk := &TClunk{FCall{Tclunk, tag}, fid}
	fmt.Printf(">>> %s\n", clunk)
	cli.conn.Write(clunk.Compose())
	ifcall, err := ParseCall(cli.conn)
	fmt.Printf("<<< %s\n", ifcall)
	if err != nil {
		return err
	}
	if ifcall.GetFCall().Ctype != Rclunk {
		if ifcall.GetFCall().Ctype == Rerror {
			message := ifcall.(*RError).Ename
			return errors.New(message)
		}
		return errors.New("Unknown Server Protocol Error.")
	}
	return nil
}

func (cli *Client) Stat(path string) (*Stat, error) {
	tag := cli.allocTag()
	defer cli.freeTag(tag)

	fid, err := cli.walk(path)
	if err != nil {
		return nil, err
	}
	defer cli.clunk(fid)
	defer cli.freeFid(fid)

	read := &TStat{FCall{Tstat, tag}, fid}
	fmt.Printf(">>> %s\n", read)
	cli.conn.Write(read.Compose())

	ifcall, err := ParseCall(cli.conn)
	fmt.Printf("<<< %s\n", ifcall)
	if err != nil {
		return nil, err
	}
	if ifcall.GetFCall().Ctype != Rstat {
		if ifcall.GetFCall().Ctype == Rerror {
			message := ifcall.(*RError).Ename
			return nil, errors.New(message)
		}
		return nil, errors.New("Unknown Server Protocol Error.")
	}
	rstat := ifcall.(*RStat)
	var stat Stat
	stat = rstat.Stat
	return &stat, nil
}

func (cli *Client) ListDir(path string) ([]Stat, error) {
	stat, err := cli.Stat(path)
	if err != nil {
		return nil, err
	}

	if stat.Qid.Qtype & (1 << 7) == 0 {
		// It's not a directory!
		return nil, errors.New("Can't List non-directory.")
	}
	
	tag := cli.allocTag()
	defer cli.freeTag(tag)

	fid, err := cli.walk(path)
	if err != nil {
		return nil, err
	}
	defer cli.clunk(fid)
	defer cli.freeFid(fid)

	file, err := cli.Open(path, Oread)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	stats := make([]Stat, 0)
	for {
		buf := make([]byte, 3000)
		n, err := file.Read(buf)
		buf = buf[:n]
		if err != nil {
			return nil, err
		}
		if n <= 0 {
			break
		}
		fmt.Printf("Got back from read buf: len: %d\n", len(buf)) 

		// Read all the stats we got on this read.
		for {
			if len(buf) == 0 {
				break
			}
			fmt.Println("Appending stat.")
			s := Stat{}
			buf, err = s.Parse(buf)
			if err != nil {
				fmt.Printf("Got err while parsing stat: len(buf): %d\n", len(buf))
				return nil, err
			}
			stats = append(stats, s)
		}
	}
	return stats, nil
}

func (cli *Client) Create(path string, fmode uint32, omode Mode) (*File9P, error) {	
	splits := strings.Split(path, "/")

	var dirpath string
	if(len(splits) > 1) {
		dirpath = strings.Join(splits[:len(splits)-1], "/")
	} else {
		dirpath = ""
	}

	fmt.Println("Walking to %s\n", dirpath) 
	
	fid, err := cli.walk(dirpath)
	if err != nil {
		return nil, err
	}

	tag := cli.allocTag()
	defer cli.freeTag(tag)

	
	name := splits[len(splits) - 1]
	if(name == "") {
		return nil, errors.New("Bad file name! \"\"")
	}
	
	create := &TCreate{FCall{Tcreate, tag}, fid, name, fmode, uint8(omode)}
	fmt.Printf(">>> %s\n", create)
	cli.conn.Write(create.Compose())

	ifcall, err := ParseCall(cli.conn)
	fmt.Printf("<<< %s\n", ifcall)
	if err != nil {
		cli.clunk(fid)
		cli.freeFid(fid)
		return nil, err
	}
	if ifcall.GetFCall().Ctype != Rcreate {
		cli.clunk(fid)
		cli.freeFid(fid)
		if ifcall.GetFCall().Ctype == Rerror {
			message := ifcall.(*RError).Ename
			return nil, errors.New(message)
		}
		return nil, errors.New("Unknown Server Protocol Error.")
	}

	iounit := ifcall.(*RCreate).Iounit
	return &File9P{fid, 0, cli, iounit}, nil
}

func (cli *Client) Open(path string, omode Mode) (*File9P, error) {
	fid, err := cli.walk(path)
	if err != nil {
		return nil, err
	}

	tag := cli.allocTag()
	defer cli.freeTag(tag)

	open := &TOpen{FCall{Topen, tag}, fid, omode}
	fmt.Printf(">>> %s\n", open)
	cli.conn.Write(open.Compose())

	ifcall, err := ParseCall(cli.conn)
	fmt.Printf("<<< %s\n", ifcall)
	if err != nil {
		cli.clunk(fid)
		cli.freeFid(fid)
		return nil, err
	}
	if ifcall.GetFCall().Ctype != Ropen {
		cli.clunk(fid)
		cli.freeFid(fid)
		if ifcall.GetFCall().Ctype == Rerror {
			message := ifcall.(*RError).Ename
			return nil, errors.New(message)
		}
		return nil, errors.New("Unknown Server Protocol Error.")
	}

	return &File9P{fid, 0, cli, ifcall.(*ROpen).Iounit}, nil

}

func (f *File9P) Read(p []byte) (n int, err error) {
	max := f.iounit
	if(max == 0) {
		max = f.cli.msize - 20
	}

	totalread := 0
	for totalread < len(p) {
	
		tocopy := uint32(len(p) - totalread)
		if(tocopy > max) {
			tocopy = max
		}

		offset := totalread

		n, err := f.SingleRead(p[offset:uint32(offset)+tocopy])
		if err != nil {
			return 0, err
		}
		if n == 0 {
			break
		}
		totalread += n
		
	}
	return totalread, nil
}

func (f *File9P) SingleRead(p []byte) (n int, err error) {
	tag := f.cli.allocTag()
	defer f.cli.freeTag(tag)
	
	read := &TRead{FCall{Tread, tag}, f.fid, f.offset, uint32(len(p))}
	fmt.Printf(">>> %s\n", read)
	f.cli.conn.Write(read.Compose())

	ifcall, err := ParseCall(f.cli.conn)
	fmt.Printf("<<< %s\n", ifcall)
	if err != nil {
		return 0, err
	}
	if ifcall.GetFCall().Ctype != Rread {
		if ifcall.GetFCall().Ctype == Rerror {
			message := ifcall.(*RError).Ename
			return 0, errors.New(message)
		}
		return 0, errors.New("Unknown Server Protocol Error.")
	}
	rread := ifcall.(*RRead)
	f.offset += uint64(rread.Count)
	copy(p, rread.Data)
	return int(rread.Count), nil
}

func (f *File9P) Write(p []byte) (n int, err error) {
	max := f.iounit
	if(max == 0) {
		max = f.cli.msize - 20
	}

	totalwritten := 0
	for totalwritten < len(p) {
	
		tocopy := uint32(len(p) - totalwritten)
		if(tocopy > max) {
			tocopy = max
		}

		offset := totalwritten

		n, err := f.SingleWrite(p[offset:uint32(offset)+tocopy])
		if err != nil {
			return 0, err
		}
		if n == 0 {
			break
		}
		totalwritten += n
		
	}
	return totalwritten, nil
}

func (f *File9P) SingleWrite(p []byte) (n int, err error) {
	tag := f.cli.allocTag()
	defer f.cli.freeTag(tag)

	write := &TWrite{FCall{Twrite, tag}, f.fid, f.offset, uint32(len(p)), p}
	fmt.Printf(">>> %s\n", write)
	f.cli.conn.Write(write.Compose())

	ifcall, err := ParseCall(f.cli.conn)
	fmt.Printf("<<< %s\n", ifcall)
	if err != nil {
		return 0, err
	}
	if ifcall.GetFCall().Ctype != Rwrite {
		if ifcall.GetFCall().Ctype == Rerror {
			message := ifcall.(*RError).Ename
			return 0, errors.New(message)
		}
		return 0, errors.New("Unknown Server Protocol Error.")
	}
	rwrite := ifcall.(*RWrite)
	return int(rwrite.Count), nil
}

func (f *File9P) Seek(offset uint64) {
	f.offset = offset
}

func (f *File9P) Close() error {
	tag := f.cli.allocTag()
	defer f.cli.freeTag(tag)

	clunk := &TClunk{FCall{Tclunk, tag}, f.fid}
	fmt.Printf(">>> %s\n", clunk)
	f.cli.conn.Write(clunk.Compose())

	ifcall, err := ParseCall(f.cli.conn)
	fmt.Printf("<<< %s\n", ifcall)
	if err != nil {
		return err
	}
	if ifcall.GetFCall().Ctype != Rclunk {
		if ifcall.GetFCall().Ctype == Rerror {
			message := ifcall.(*RError).Ename
			return errors.New(message)
		}
		return errors.New("Unknown Server Protocol Error.")
	}
	return nil
}
