package fcall

import (
	"fmt"
)

type Handler struct {
	Open func(fs *Filesystem, conn *Connection, ctx *Opencontext)
	Read func(fs *Filesystem, conn *Connection, ctx *Readcontext)
	Write func(fs *Filesystem, conn *Connection, ctx *Writecontext)
	Create func(fs *Filesystem, conn *Connection, ctx *Createcontext)
	Setup func(fs *Filesystem, conn *Connection)
}

// Internal implementation
type Opencontext struct {
	conn *Connection
	fs *Filesystem
	call *TOpen
	file *File
}

func (ctx *Opencontext) Respond() {
	response := &ROpen{FCall{Ropen, ctx.call.Tag}, ctx.file.stat.Qid, iounit}
	fmt.Println("<<< ", response)
	ctx.conn.Conn.Write(response.Compose())
}

func (ctx *Opencontext) GetFile() interface{} {
	return ctx.file
}

func (ctx *Opencontext) AddFile(mode uint32, length uint64, name string, parent interface{}) {
	
}

type Readcontext struct {
	conn *Connection
	fs *Filesystem
}

func (ctx *Readcontext) Respond() {

}

func (ctx *Readcontext) GetFile() interface{} {
	return nil
}

func (ctx *Readcontext) AddFile(mode uint32, length uint64, name string, parent interface{}) {
	
}

type Writecontext struct {
	conn *Connection
	fs *Filesystem
}

func (ctx *Writecontext) Respond() {
	
}

func (ctx *Writecontext) GetFile() interface{} {
	return nil
}

func (ctx *Writecontext) AddFile(mode uint32, length uint64, name string, parent interface{}) {
	
}

type Createcontext struct {
	conn *Connection
	fs *Filesystem
}

func (ctx *Createcontext) Respond() {
	
}

func (ctx *Createcontext) GetFile() interface{} {
	return nil
}

func (ctx *Createcontext) AddFile(mode uint32, length uint64, name string, parent interface{}) {
	
}
