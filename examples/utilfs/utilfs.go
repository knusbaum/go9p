package main

import (
	"fmt"
	"github.com/knusbaum/go9p"
	"net"
	"crypto/rand"
	"time"
)



// Fid -> data
var data map[uint32][]byte
var funcs map[string] func(*go9p.ReadContext)

func Open(ctx *go9p.OpenContext) {
		ctx.Respond()
}

func Read(ctx *go9p.ReadContext) {
	if funcs[ctx.File.Path] != nil {
		funcs[ctx.File.Path](ctx)
	} else {
		ctx.Respond(nil)
	}
}

func Close(ctx *go9p.Ctx) {
	delete(data, ctx.Fid)
}

func Setup(ctx *go9p.UpdateContext) {
	root := ctx.File

	timefile := ctx.AddFile(0444, 0, "time", "root", root)
	funcs[timefile.Path] = func(ctx *go9p.ReadContext) {
		if data[ctx.Fid] == nil {
			data[ctx.Fid] = []byte(time.Now().String() + "\n")
		}
		out := go9p.SliceForRead(ctx, data[ctx.Fid])
		ctx.Respond(out)
	}

	random := ctx.AddFile(0444, 0, "random", "root", root)
	funcs[random.Path] = func(ctx *go9p.ReadContext) {
		data := make([]byte, ctx.Count)
		rand.Reader.Read(data)
		ctx.Respond(data)
	}
}

func main() {
	data = make(map[uint32][]byte, 0)
	funcs = make(map[string] func(*go9p.ReadContext), 0)
	srv := &go9p.Server{
		Open:   Open,
		Read:   Read,
		Write:  nil,
		Close:  Close,
		Create: nil,
		Setup:  Setup}
	fmt.Println("Starting server...")

	listener, error := net.Listen("tcp", "0.0.0.0:9999")
	if error != nil {
		fmt.Println("Failed to listen: ", error)
		return
	}

	srv.Serve(listener)
}
