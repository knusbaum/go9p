package go9p

import (
	"fmt"
	"net"
	"time"
)


// Server struct contains functions which are
// called on file open, read, write, and create
// See the definitions of the various context types.
// All "inherit" Ctx and so the Fail() and AddFile() functions.
// Each implements a Respond() function. The parameters and behavior
// of each changes among the contexts
//
// Setup is called once the server is up and running, and is
// the appropriate place to add any initial files to the server.
type Server struct {
	Open   func(ctx *Opencontext)
	Read   func(ctx *Readcontext)
	Write  func(ctx *Writecontext)
	Create func(ctx *Createcontext)
	Setup  func(ctx *Ctx)
}

// Serve serves the 9P file server srv.
// see Server
func Serve(srv *Server) {

	var mode uint32
	var i uint32
	for i = 0; i < 9; i++ {
		mode |= (1 << i)
	}
	mode = mode ^ (1 << 1) // o-w

	fs := initializeFs()
	root := fs.addFile("/", Stat{
		Stype:  0,
		Dev:    0,
		Qid:    fs.allocQid(1 << 7),
		Mode:   mode | (1 << 31) | (1 << 1), // Add dir bit and o+w
		Atime:  uint32(time.Now().Unix()),
		Mtime:  uint32(time.Now().Unix()),
		Length: 0,
		Name:   "/",
		Uid:    "root",
		Gid:    "root",
		Muid:   "root"},
		nil)

	if srv.Setup != nil {
		ctx := &Ctx{nil, &fs, nil, 0, root}
		srv.Setup(ctx)
	}

	listener, error := net.Listen("tcp", "0.0.0.0:9999")
	if error != nil {
		fmt.Println("Failed to listen: ", error)
		return
	}

	for {
		go9conn := connection{}
		err := go9conn.accept(listener)

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

// Ctx is the base context. All other contexts
// embed this type and therefore inherit the Fail()
// and AddFile() functions.
//
// Fid is the file descriptor on which the request
// is operating.
// File is the file associated with the request. For Setup,
// File is the root directory, and for Create, it's the
// directory in which the client is trying to create a file.
// For Open, Read, and Write, File is the file to be opened,
// read, or written.
type Ctx struct {
	conn *connection
	fs   *filesystem
	call *FCall
	Fid  uint32
	File *File
}

// Fail - Call this on the context when a request can't
// be fulfilled. The string is passed to the client as
// an explanation of the failure that occurred.
func (ctx *Ctx) Fail(s string) {
	response := &RError{FCall{rerror, ctx.call.Tag}, s}
	fmt.Println("<<< ", response)
	ctx.conn.Conn.Write(response.Compose())
}

// AddFile - Use this function to add a file to the server.
// mode - The mode for the new file
// length - The length of the new file
// name - The file's name (not the full path)
// owner - The owner of the new file
// parent - The directory that the file will be created under.
// This function returns a pointer to the newly created file.
func (ctx *Ctx) AddFile(mode uint32, length uint64, name string, owner string, parent *File) *File {
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
	if mode&(1<<31) != 0 {
		// It's a directory.
		qidtype = (1 << 7)
	}
	return ctx.fs.addFile(path,
		Stat{
			Stype:  0,
			Dev:    0,
			Qid:    ctx.fs.allocQid(qidtype),
			Mode:   mode,
			Atime:  uint32(time.Now().Unix()),
			Mtime:  uint32(time.Now().Unix()),
			Length: length,
			Name:   name,
			Uid:    owner,
			Gid:    parent.Stat.Gid,
			Muid:   owner},
		parent)
}

// Username - Returns the user associated with the action.
// This will be the empty string on Setup - no user has connected.
func (ctx *Ctx) Username() string {
	if ctx.conn != nil {
		return ctx.conn.uname
	}
	return ""
}

// Opencontext - The context passed to the Open callback
// in Server.
// Mode is the requested open mode for the file.
// Mode can be ignored unless you want to do some special
// logic. The server handles the common case - it won't call
// Write on a file not opened for writing, etc.
type Opencontext struct {
	Ctx
	Mode uint8
}

// Respond - Tells the client the open operation was successful.
func (ctx *Opencontext) Respond() {
	ctx.conn.setFidOpenmode(ctx.Fid, ctx.Mode)
	response := &ROpen{FCall{ropen, ctx.call.Tag}, ctx.File.Stat.Qid, iounit}
	fmt.Println("<<< ", response)
	ctx.conn.Conn.Write(response.Compose())
}

// Readcontext - The context passed to the Read callback
// in Server.
// Offset is the offset into the file that is requested
// Count is the number of bytes requested.
type Readcontext struct {
	Ctx
	Offset uint64
	Count  uint32
}

// Respond - Sends requested data back to the client.
func (ctx *Readcontext) Respond(data []byte) {
	response := &RRead{FCall{rread, ctx.call.Tag}, uint32(len(data)), data}
	fmt.Println("<<< ", response)
	ctx.conn.Conn.Write(response.Compose())
}

// Writecontext - The context passed to the Write callback
// in Server.
// Data - the data from the client that they want to write
// to the file
// Offset - the offset within the file that the data is to
// be written at
// Count - the number of bytes to be written. (this should
// correspond with len(Data)
type Writecontext struct {
	Ctx
	Data   []byte
	Offset uint64
	Count  uint32
}

// Respond - Tells the client the number of bytes that
// were successfully written. This should be identical to
// Writecontext.Count. The client might consider it to be
// an error if the bytes written differs from the requested
// amount.
func (ctx *Writecontext) Respond(count uint32) {
	response := &RWrite{FCall{rwrite, ctx.call.Tag}, count}
	fmt.Println("<<< ", response)
	ctx.conn.Conn.Write(response.Compose())
}

// Createcontext - The context passed to the Create callback
// in Server.
// NewPath - the full path of the new file (including Name)
// Name - the name of the new file.
// Perm - the permissions of the file. Lower 9 bits are
// Unix-style permissions (user/group/other rwxrwxrwx). The high
// bit (1 << 31) is the directory flag.
type Createcontext struct {
	Ctx
	NewPath string
	Name    string
	Perm    uint32
	Mode    uint8
}

// Respond - Creates the file requested and sets the length.
// Returns the new file.
func (ctx *Createcontext) Respond(length uint64) *File {
	newfile :=
		ctx.fs.addFile(ctx.NewPath,
			Stat{
				Stype:  0,
				Dev:    0,
				Qid:    ctx.fs.allocQid(uint8(ctx.Perm >> 24)),
				Mode:   ctx.Perm,
				Atime:  uint32(time.Now().Unix()),
				Mtime:  uint32(time.Now().Unix()),
				Length: length,
				Name:   ctx.Name,
				Uid:    ctx.conn.uname,
				Gid:    ctx.File.Stat.Gid,
				Muid:   ctx.conn.uname},
			ctx.File)
	ctx.conn.setFidPath(ctx.Fid, ctx.NewPath)
	ctx.conn.setFidOpenmode(ctx.Fid, Ordwr)

	response := &RCreate{FCall{rcreate, ctx.call.Tag}, newfile.Stat.Qid, iounit}
	fmt.Println("<<< ", response)
	ctx.conn.Conn.Write(response.Compose())
	return newfile
}
