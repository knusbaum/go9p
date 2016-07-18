package go9p

import (
	"github.com/knusbaum/go9p/fcall"
	"time"
	"fmt"
	"net"
)

type Srv interface {
	Open() func(ctx Context)
	Read() func(ctx Context)
	Write() func(ctx Context)
	Create() func(ctx Context)
	Setup() func(ctx Context)
}

//type CtxFile interface {}

type Context interface {
	Respond()
	GetFile() interface{}
	AddFile(mode uint32, length uint64, name string, parent interface{})
}

type BaseServer struct {
	OpenFn func(ctx Context)
	ReadFn func(ctx Context)
	WriteFn func(ctx Context)
	CreateFn func(ctx Context)
	SetupFn func(ctx Context)
}

func (srv *BaseServer) Open() func(ctx Context) {
	return srv.OpenFn
}

func (srv *BaseServer) Read() func(ctx Context) {
	return srv.ReadFn
}

func (srv *BaseServer) Write() func(ctx Context) {
	return srv.WriteFn
}

func (srv *BaseServer) Create() func(ctx Context) {
	return srv.CreateFn
}

func (srv *BaseServer) Setup() func(ctx Context) {
	return srv.SetupFn
}

func makeHandler(conn fcall.Connection, srv Srv) fcall.Handler {
	var open func(fs *fcall.Filesystem, conn *fcall.Connection, ctx *fcall.Opencontext)
	var read func(fs *fcall.Filesystem, conn *fcall.Connection, ctx *fcall.Readcontext)
	var write func(fs *fcall.Filesystem, conn *fcall.Connection, ctx *fcall.Writecontext)
	var create func(fs *fcall.Filesystem, conn *fcall.Connection, ctx *fcall.Createcontext)
	var setup func(fs *fcall.Filesystem, conn *fcall.Connection)

	sopen := srv.Open()
	if sopen != nil {
		open = 
			func(fs *fcall.Filesystem, conn *fcall.Connection, ctx *fcall.Opencontext) {
			sopen(ctx)
		}
	}

	sread := srv.Read()
	if sread != nil {
		read = func(fs *fcall.Filesystem, conn *fcall.Connection, ctx *fcall.Readcontext) {
			sread(ctx)
		}
	}

	swrite := srv.Write()
	if swrite != nil {
		write = func(fs *fcall.Filesystem, conn *fcall.Connection, ctx *fcall.Writecontext) {
			swrite(ctx)
		}
	}

	screate := srv.Create()
	if screate != nil {
		create = func(fs *fcall.Filesystem, conn *fcall.Connection, ctx *fcall.Createcontext) {
			screate(ctx)
		}
	}

	ssetup := srv.Setup()
	if ssetup != nil {
		setup = func(fs *fcall.Filesystem, conn *fcall.Connection) {
			ssetup(nil)
		}
	}
	
	return fcall.Handler{
		open,
		read,
		write,
		create,
		setup}
}

func Serve(srv Srv) {
	
	var mode uint32
	var i uint32
	for i = 0; i < 9; i++ {
		mode |= (1<<i)
	}
	mode = mode ^ (1<<1); // o-w

	fs := fcall.InitializeFs()
	fs.AddFile("/", fcall.Stat{
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
		go9conn := fcall.Connection{}
		h := makeHandler(go9conn, srv)
		err := go9conn.Accept(listener)

		if err != nil {
			fmt.Println("Failed to accept: ", err)
			return
		}
		for {
			fc, err := fcall.ParseCall(go9conn.Conn)
			if err != nil {
				fmt.Println("Failed to parse call: ", err)
				if fc != nil {
					go9conn.Conn.Write(fc.Compose())
					continue
				}
				break
			}
			
			fmt.Println(">>> ", fc)
			reply := fc.Reply(&fs, &go9conn, h)
			if reply != nil {
				fmt.Println("<<< ", reply)
				go9conn.Conn.Write(reply.Compose())
			}
		}
	}
}
