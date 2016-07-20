package go9p

import (
//	"github.com/knusbaum/go9p/fcall"
	"time"
	"fmt"
	"net"
)

type Context interface {
	//Respond()
	Fail(s string)
	AddFile(mode uint32, length uint64, name string, parent *File) *File
}

type Server struct {
	Open func(ctx *Opencontext)
	Read func(ctx *Readcontext)
	Write func(ctx *Writecontext)
	Create func(ctx *Createcontext)
	Setup func(ctx Context)
}

func Serve(srv *Server) {
	
	var mode uint32
	var i uint32
	for i = 0; i < 9; i++ {
		mode |= (1<<i)
	}
	mode = mode ^ (1<<1); // o-w

	fs := InitializeFs()
	fs.AddFile("/", Stat{
		Stype: 0,
		Dev: 0,
		Qid: fs.AllocQid(1 << 7),
		Mode: mode | (1<<31) | (1<<1), // Add dir bit and o+w
		Atime: uint32(time.Now().Unix()),
		Mtime: uint32(time.Now().Unix()),
		Length: 0,
		Name: "/",
		Uid: "root",
		Gid: "root",
		Muid: "root"},
		nil)

	listener, error := net.Listen("tcp", "0.0.0.0:9999")
	if error != nil {
		fmt.Println("Failed to listen: ", error)
		return
	}

	for {
		go9conn := Connection{}
		//h := makeHandler(go9conn, srv)
		err := go9conn.Accept(listener)

		if err != nil {
			fmt.Println("Failed to accept: ", err)
			return
		}
		for {
			fc, err := ParseCall(go9conn.Conn)
			if err != nil {
				fmt.Println("Failed to parse call: ", err)
				if fc != nil {
					go9conn.Conn.Write(fc.Compose())
					continue
				}
				break
			}
			
			fmt.Println(">>> ", fc)
			reply := fc.Reply(&fs, &go9conn, srv)
			if reply != nil {
				fmt.Println("<<< ", reply)
				go9conn.Conn.Write(reply.Compose())
			}
		}
	}
}



/* FCALL SERVER.GO */
type Ctx struct {
	conn *Connection
	fs *Filesystem
	call *FCall
	Fid uint32
	File *File
}

func (ctx *Ctx) Fail(s string) {
	response := &RError{FCall{Rerror, ctx.call.Tag}, s}
	fmt.Println("<<< ", response)
	ctx.conn.Conn.Write(response.Compose())
}

func (ctx *Ctx) AddFile(mode uint32, length uint64, name string, parent *File) *File{
	if parent == nil {
		return nil
	}
	path := ""
	if parent.Path == "/" {
		path = parent.Path + name
	} else {
		path = parent.Path + "/" + name
	}
	var qidtype uint8
	if mode & (1 << 31) != 0 {
		// It's a directory.
		qidtype = (1 << 7)
	}
	return ctx.fs.AddFile(path,
		Stat{
			Stype: 0,
			Dev: 0,
			Qid: ctx.fs.AllocQid(qidtype),
			Mode: mode,
			Atime: uint32(time.Now().Unix()),
			Mtime: uint32(time.Now().Unix()),
			Length: length,
			Name: name,
			Uid: ctx.conn.uname,
			Gid: parent.Stat.Gid,
			Muid: ctx.conn.uname},
		parent)
}

type Opencontext struct {
	Ctx
	Mode uint8
}

func (ctx *Opencontext) Respond() {
	ctx.conn.SetFidOpenmode(ctx.Fid, ctx.Mode)
	response := &ROpen{FCall{Ropen, ctx.call.Tag}, ctx.File.Stat.Qid, iounit}
	fmt.Println("<<< ", response)
	ctx.conn.Conn.Write(response.Compose())
}


type Readcontext struct {
	Ctx
	Offset uint64
	Count uint32
}

func (ctx *Readcontext) Respond(data []byte) {
	response := &RRead{FCall{Rread, ctx.call.Tag}, uint32(len(data)), data}
	fmt.Println("<<< ", response)
	ctx.conn.Conn.Write(response.Compose())
}

type Writecontext struct {
	Ctx
	Data []byte
	Offset uint64
	Count uint32
}

func (ctx *Writecontext) Respond(count uint32) {
	response := &RWrite{FCall{Rwrite, ctx.call.Tag}, count}
	fmt.Println("<<< ", response)
	ctx.conn.Conn.Write(response.Compose())
}

type Createcontext struct {
	Ctx
	NewPath string
	Name string
	Perm uint32
	Mode uint8
}

func (ctx *Createcontext) Respond(length uint64) {
	newfile :=
		ctx.fs.AddFile(ctx.NewPath,
		Stat{
			Stype: 0,
			Dev: 0,
			Qid: ctx.fs.AllocQid(uint8(ctx.Perm >> 24)),
			Mode: ctx.Perm,
			Atime: uint32(time.Now().Unix()),
			Mtime: uint32(time.Now().Unix()),
			Length: length,
			Name: ctx.Name,
			Uid: ctx.conn.uname,
			Gid: ctx.File.Stat.Gid,
			Muid: ctx.conn.uname},
		ctx.File)
	ctx.conn.SetFidPath(ctx.Fid, ctx.NewPath)
	ctx.conn.SetFidOpenmode(ctx.Fid, Ordwr)

	response := &RCreate{FCall{Rcreate, ctx.call.Tag}, newfile.Stat.Qid, iounit}
	fmt.Println("<<< ", response)
	ctx.conn.Conn.Write(response.Compose())
}
