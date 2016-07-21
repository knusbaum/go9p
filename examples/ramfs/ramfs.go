package main

import (
	"fmt"
	"net"
	"github.com/knusbaum/go9p"
)

var data map[string][]byte

func Open(ctx *go9p.Opencontext) {
	ctx.Respond()
}

func Read(ctx *go9p.Readcontext) {
	if ctx.Offset  >= ctx.File.Stat.Length {
		ctx.Respond(nil)
		return
	}

	count := uint64(ctx.Count)
	if ctx.Offset + count > ctx.File.Stat.Length {
		count = ctx.File.Stat.Length - ctx.Offset
	}
	response := data[ctx.File.Path][ctx.Offset:ctx.Offset + count]
	ctx.Respond(response)
}

func Write(ctx *go9p.Writecontext) {
	contents := data[ctx.File.Path]
	if ctx.Offset + uint64(ctx.Count) > uint64(len(contents)) {
		// Not enough room in contents. Extend.
		newlen := ctx.Offset + uint64(ctx.Count)
		ctx.File.Stat.Length = newlen
		newbuff := make([]byte, newlen - uint64(len(contents)))
		contents = append(contents, newbuff...)
	}

	copy(contents[ctx.Offset:ctx.Offset+uint64(ctx.Count)], ctx.Data)
	data[ctx.File.Path] = contents
	ctx.Respond(ctx.Count)
}

func Create(ctx *go9p.Createcontext) {
	data[ctx.NewPath] = make([]byte, 0)
	ctx.Respond(0)
}

func main() {
	data = make(map[string][]byte, 0)
	srv := &go9p.Server{
		Open: Open,
		Read: Read,
		Write: Write,
		Create: Create,
		Setup: nil}
	fmt.Println("Starting server...")

	listener, error := net.Listen("tcp", "0.0.0.0:9999")
	if error != nil {
		fmt.Println("Failed to listen: ", error)
		return
	}

	srv.Serve(listener)
}
