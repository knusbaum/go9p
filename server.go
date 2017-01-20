// Package go9p is a Go implementation of the 9P2000 protocol.
// It imlements a parser and composer for 9P2000 messages as well as
// a server (See the Server type).
//
// The server requires users to implement callbacks for certain operations.
// Open, Read, Write, Create, and Setup. See the docs for the *context structs
// for explanations of how to appropriately respond to these events.
//
// IFCall, FCall, and all the T* and R* types are internal types used for parsing,
// etc. They're left exposed if you want to do some less structured 9P programming.
//
// Details of the 9P2000 protocol can be found here: http://knusbaum.inlisp.org/res/rfc9p2000.html
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
	Open    func(ctx *OpenContext)
	Read    func(ctx *ReadContext)
	DirRead func(ctx *DirReadContext) // CANNOT SPAWN GOROUTINES USING UPDATE CONTEXT!
	Write   func(ctx *WriteContext)
	Close   func(ctx *Ctx)
	Create  func(ctx *CreateContext)
	Remove  func(ctx *RemoveContext)
	Setup   func(ctx *UpdateContext)
	Auth    func(ctx *AuthContext, in <-chan []byte, out chan<- []byte) // Run in a separate goroutine
	// Internal use
	outgoing chan outgoing
}

type outgoing struct {
	conn *connection
	msg []byte
}

type incoming struct {
	conn *connection
	call IFCall
}

func process9PConnection(conn *connection, ch chan incoming) {
	for {
		fc, err := ParseCall(conn.Conn)
		if err != nil {
			fmt.Println("Failed to parse call: ", err)
			if fc != nil {
				conn.Conn.Write(fc.Compose())
				continue
			}
			return
		}
		ch <- incoming{conn, fc}
	}
}

func acceptNewConnections(listener net.Listener, ch chan *connection) {
	for {
		fmt.Println("Accepting connections!")
		go9conn := &connection{}
		err := go9conn.accept(listener)
		if err != nil {
			fmt.Println("Failed to accept: ", err)
			close(ch)
			return
		}
		fmt.Println("Putting new conn on channel.")
		ch <- go9conn
	}
}

// Serve serves the 9P2000 file server.
func (srv *Server) Serve(listener net.Listener) {

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
		ctx := &UpdateContext{Ctx{nil, &fs, nil, 0, root}}
		srv.Setup(ctx)
	}

	newConns := make(chan *connection)
	incomingCalls := make(chan incoming)
	srv.outgoing = make(chan outgoing)
	go acceptNewConnections(listener, newConns)

	for {
		select {
		case conn := <-newConns:
			if conn == nil {
				// Stop serving immediately. (add some cleanup here later)
				return
			}
			go process9PConnection(conn, incomingCalls)

		case incoming := <-incomingCalls:
			conn := incoming.conn
			fc := incoming.call
			fmt.Println(">>> ", fc)
			reply := fc.Reply(&fs, conn, srv)
			if reply != nil {
				fmt.Println("<<< ", reply)
				conn.Conn.Write(reply.Compose())
			}
		// This is only used for auth
		case outgoing := <-srv.outgoing:
			fmt.Println("writing outgoing message!")
			conn := outgoing.conn
			msg := outgoing.msg
			conn.Conn.Write(msg)

		case update := <-fs.updateChan:
			uctx := &UpdateContext{*update.originalCtx}
			update.fn(uctx)
			uctx.conn.setDirContents(uctx.Fid, uctx.File.composeSubfiles())
		}
	}
}

// Ctx is the base context. All other contexts
// embed this type and therefore inherit the Fail()
// and UpdateFS() functions.
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
	response := &RError{FCall{Rerror, ctx.call.Tag}, s}
	fmt.Println("<<< ", response)
	ctx.conn.Conn.Write(response.Compose())
}

// Username - Returns the user associated with the action.
// This will be the empty string on Setup - no user has connected.
func (ctx *Ctx) Username() string {
	if ctx.conn != nil {
		return ctx.conn.uname
	}
	return ""
}

func (ctx *Ctx) FileByPath(path string) *File {
	return ctx.fs.files[path]
}

// UpdateFS runs the argument function, passing it an update context
// so that it can make modifications to the filesystem.
// fn is not run immediately, but passed to the main routine.
// This removes the need for synchronization. You can be sure
// that only one routine is modifying the filesystem structure at
// any one time.
func (ctx *Ctx) UpdateFS(fn func(*UpdateContext)) {
	// This needs to execute in its own goroutine
	// since UpdateFS can be called in the main thread
	// and cause deadlock if updateChan is full.
	go func() {
		fmt.Println("Enqueueing update fn")
		ctx.fs.updateChan <- update{ctx, fn}
		fmt.Println("Done Enqueueing update fn")
	}()
}

// UpdateContext is the context given to functions passed to UpdateFS.
// UpdateContext has functions that allow a user to modify the filesystem,
// adding, removing files, etc.
type UpdateContext struct {
	Ctx
}

// AddFile - Use this function to add a file to the server.
// mode - The mode for the new file
// length - The length of the new file
// name - The file's name (not the full path)
// owner - The owner of the new file
// parent - The directory that the file will be created under.
// This function returns a pointer to the newly created file.
func (ctx *UpdateContext) AddFile(mode uint32, length uint64, name string, owner string, parent *File) *File {
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

// RemoveFile - Remove a file from the filesystem.
func (ctx *UpdateContext) RemoveFile(f *File) {
	ctx.fs.removeFile(f)
}

// OpenContext - The context passed to the Open callback
// in Server.
// Mode is the requested open mode for the file.
// Mode can be ignored unless you want to do some special
// logic. The server handles the common case - it won't call
// Write on a file not opened for writing, etc.
type OpenContext struct {
	Ctx
	Mode uint8
}

// Respond - Tells the client the open operation was successful.
func (ctx *OpenContext) Respond() {
	ctx.conn.setFidOpenmode(ctx.Fid, ctx.Mode)
	ctx.conn.setFidOpenoffset(ctx.Fid, ctx.File.Stat.Length)
	if ctx.File.Stat.Mode&(1<<31) != 0 {
		// If this is a directory, write out all subfile stats now so we have a consistent
		// view of the directory throughout the life of the Fid
		ctx.conn.setDirContents(ctx.Fid, ctx.File.composeSubfiles())
	}
	response := &ROpen{FCall{Ropen, ctx.call.Tag}, ctx.File.Stat.Qid, iounit}
	fmt.Println("<<< ", response)
	ctx.conn.Conn.Write(response.Compose())
}

// ReadContext - The context passed to the Read callback
// in Server.
// Offset is the offset into the file that is requested
// Count is the number of bytes requested.
type ReadContext struct {
	Ctx
	Offset uint64
	Count  uint32
}

// Respond - Sends requested data back to the client.
func (ctx *ReadContext) Respond(data []byte) {
	response := &RRead{FCall{Rread, ctx.call.Tag}, uint32(len(data)), data}
	fmt.Println("<<< ", response)
	ctx.conn.Conn.Write(response.Compose())
}

type DirReadContext struct {
	Ctx
	read *TRead
}

// Respond - Marks the directory ready for read by the client.
func (ctx *DirReadContext) Respond() {
	if ctx.File.Stat.Mode&(1<<31) != 0 {
		// If this is a directory, write out all subfile stats now so we have a consistent
		// view of the directory throughout the life of the Fid
		ctx.conn.setDirContents(ctx.Fid, ctx.File.composeSubfiles())
	}
	response := doDirRead(ctx.read, ctx.File, ctx.conn)
	fmt.Println("<<< ", response)
	ctx.conn.Conn.Write(response.Compose())
}

// SliceForRead - Given a read context and a slice representing the full file contents,
// return a slice at offset ctx.Offset of ctx.Count bytes. Handles cases where file is
// nil, offset + count > len(file), etc. The return value is ready to be passed to
// ctx.Respond.
func SliceForRead(ctx *ReadContext, file []byte) []byte {
	flen := uint64(len(file))

	if file == nil {
		return nil
	}
	if ctx.Offset >= flen {
		return nil
	}

	count := uint64(ctx.Count)
	if ctx.Offset+count > flen {
		count = flen - ctx.Offset
	}

	response := file[ctx.Offset : ctx.Offset+count]
	return response
}

// WriteContext - The context passed to the Write callback
// in Server.
// Data - the data from the client that they want to write
// to the file
// Offset - the offset within the file that the data is to
// be written at
// Count - the number of bytes to be written. (this should
// correspond with len(Data)
type WriteContext struct {
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
func (ctx *WriteContext) Respond(count uint32) {
	response := &RWrite{FCall{Rwrite, ctx.call.Tag}, count}
	fmt.Println("<<< ", response)
	ctx.conn.Conn.Write(response.Compose())
}

// CreateContext - The context passed to the Create callback
// in Server.
// NewPath - the full path of the new file (including Name)
// Name - the name of the new file.
// Perm - the permissions of the file. Lower 9 bits are
// Unix-style permissions (user/group/other rwxrwxrwx). The high
// bit (1 << 31) is the directory flag.
type CreateContext struct {
	Ctx
	NewPath string
	Name    string
	Perm    uint32
	Mode    uint8
}

// Respond - Creates the file requested and sets the length.
// Returns the new file.
func (ctx *CreateContext) Respond(length uint64) *File {
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

	response := &RCreate{FCall{Rcreate, ctx.call.Tag}, newfile.Stat.Qid, iounit}
	fmt.Println("<<< ", response)
	ctx.conn.Conn.Write(response.Compose())
	return newfile
}

// RemoveContext - The context passed to the Create callback
// in Server.
type RemoveContext struct {
	Ctx
}

// Respond removes the file
func (ctx *RemoveContext) Respond() {
	ctx.fs.removeFile(ctx.File)
	response := &RRemove{FCall{Rremove, ctx.call.Tag}}
	fmt.Println("<<< ", response)
	ctx.conn.Conn.Write(response.Compose())
}

// AuthContext - The context given to the user's authentication functions
type AuthContext struct {
	Ctx
}

func (ctx *AuthContext) SetAuthenticated(b bool) {
	ctx.conn.authenticated = b
}
