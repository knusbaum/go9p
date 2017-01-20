// A memory-backed filesystem for storing files.
// The only special file is /overalloc, which
// lists the unused space in each file's buffer.
// Try commenting out the code in the Close function
// to see how that affects the overallocation numbers.
package main

import (
	"fmt"
	"github.com/knusbaum/go9p"
	"net"
	"os"
	"runtime/trace"
	"os/signal"
	"syscall"
)

// Path -> data
// Holds file data associated with a path.
var data map[string][]byte
var fidToData map[uint32][]byte

func Open(ctx *go9p.OpenContext) {
	if ctx.File.Path == "/overalloc" {
		allocs := make([]byte, 1)
		for _, buff := range data {
			diff := fmt.Sprintf("%d\n", cap(buff) - len(buff))
			allocs = append(allocs, []byte(diff)...)
		}
		fidToData[ctx.Fid] = allocs
	}
	ctx.Respond()
}

func Read(ctx *go9p.ReadContext) {
	if ctx.File.Path == "/overalloc" {
		ctx.Respond(go9p.SliceForRead(ctx, fidToData[ctx.Fid]))
	} else {
		ctx.Respond(go9p.SliceForRead(ctx, data[ctx.File.Path]))
	}
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

func Close(ctx *go9p.Ctx) {
	// When the user closes the file, let's trim the
	// extra capacity off the end of the file's buffer
	// if it's larger than, say, 1KB
	buffer := data[ctx.File.Path]
	if buffer != nil && cap(buffer) - len(buffer) > 1000 {
		newbuff := make([]byte, len(buffer))
		copy(newbuff, buffer)
		data[ctx.File.Path] = newbuff
	}
}

func Create(ctx *go9p.CreateContext) {
	// Set up an empty buffer for path.
	data[ctx.NewPath] = make([]byte, 0)
	ctx.Respond(0)
}

func Remove(ctx *go9p.RemoveContext) {
	delete(data, ctx.File.Path)
	ctx.Respond()
}

func Setup(ctx *go9p.UpdateContext) {
	root := ctx.File
	ctx.AddFile(0444, 0, "overalloc", "root", root)
}

func main() {
	data = make(map[string][]byte, 0)
	fidToData = make(map[uint32][]byte, 0)
	srv := &go9p.Server{
		Open:   Open,
		Read:   Read,
		Write:  Write,
		Close:  Close,
		Create: Create,
		Remove: Remove,
		Setup:  Setup}
	fmt.Println("Starting server...")

	listener, error := net.Listen("tcp", "0.0.0.0:9999")
	if error != nil {
		fmt.Println("Failed to listen: ", error)
		return
	}
	f, err := os.Create("trace.grt")
	defer f.Close()
	if err != nil {
		fmt.Printf("Failed to open file for trace output: %s\n", err)
	} else {
		c := make(chan os.Signal, 2)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			trace.Stop()
			f.Close()
			os.Exit(1)
		}()
		trace.Start(f)
//		defer trace.Stop()
	}
	srv.Serve(listener)
}
