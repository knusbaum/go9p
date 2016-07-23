package main

import (
	"fmt"
	"github.com/knusbaum/go9p"
	"net"
)

var data map[string][]byte

func Open(ctx *go9p.OpenContext) {
	ctx.Respond()
}

func Read(ctx *go9p.ReadContext) {
	ctx.Respond(go9p.SliceForRead(ctx, data[ctx.File.Path]))
}

func Write(ctx *go9p.WriteContext) {
	contents := data[ctx.File.Path]
	if ctx.Offset+uint64(ctx.Count) > uint64(len(contents)) {
		// Not enough room in contents. Extend.
		newlen := ctx.Offset + uint64(ctx.Count)
		ctx.File.Stat.Length = newlen
		newbuff := make([]byte, newlen-uint64(len(contents)))
		contents = append(contents, newbuff...)
	}

	copy(contents[ctx.Offset:ctx.Offset+uint64(ctx.Count)], ctx.Data)
	data[ctx.File.Path] = contents
	ctx.Respond(ctx.Count)
}

func Create(ctx *go9p.CreateContext) {
	data[ctx.NewPath] = make([]byte, 0)
	ctx.Respond(0)
}

func main() {
	data = make(map[string][]byte, 0)
	srv := &go9p.Server{
		Open:   Open,
		Read:   Read,
		Write:  Write,
		Create: Create,
		Setup:  nil}
	fmt.Println("Starting server...")

	listener, error := net.Listen("tcp", "0.0.0.0:9999")
	if error != nil {
		fmt.Println("Failed to listen: ", error)
		return
	}

	srv.Serve(listener)
}
