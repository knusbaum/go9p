// This is a sample filesystem that serves a couple "utilities"
// There's /time, which when read, will return a human-readable
// string of the current time.
// There's also /random, which is a file of infinite-length
// containing random bytes.
// Finally, there's /events, which records all of the high-level
// callbacks invoked on the Server struct.
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

// Path -> handler
// Holds handler functions for the various files.
var funcs map[string] func(*go9p.ReadContext)

// Stores a log of events that occur on the FS.
// Available for reading at /events
var eventFile *go9p.File
var eventData []byte

// Add an event to the event log.
func addEvent(s string) {
	eventData = append(eventData, []byte(s + "\n")...)
	eventFile.Stat.Length = uint64(len(eventData))
}

func Open(ctx *go9p.OpenContext) {
	addEvent(fmt.Sprintf("%s: Open: [%s]", time.Now(), ctx.File.Path))
	ctx.Respond()
}

func Read(ctx *go9p.ReadContext) {
	// Don't log read events on the /events file,
	// but log everything else.
	if ctx.File.Path != "/events" {
		addEvent(fmt.Sprintf("%s: Read: [%s] Offset: %d, Count: %d", time.Now(), ctx.File.Path, ctx.Offset, ctx.Count))
	}

	// Get the handler for the path and call it,
	// or respond with zero bytes.
	if funcs[ctx.File.Path] != nil {
		funcs[ctx.File.Path](ctx)
	} else {
		ctx.Respond(nil)
	}
}

func Close(ctx *go9p.Ctx) {
	// When a file is closed, delete any buffered data associated with
	// the Fid.
	addEvent(fmt.Sprintf("%s: Close: [%s]", time.Now(), ctx.File.Path))
	delete(data, ctx.Fid)
}

func Setup(ctx *go9p.UpdateContext) {
	root := ctx.File

	timefile := ctx.AddFile(0444, 0, "time", "root", root)
	funcs[timefile.Path] = func(ctx *go9p.ReadContext) {
		// If this is the first read call, get the time and
		// buffer it for the opened Fid.
		if data[ctx.Fid] == nil {
			data[ctx.Fid] = []byte(time.Now().String() + "\n")
		}
		out := go9p.SliceForRead(ctx, data[ctx.Fid])
		ctx.Respond(out)
	}

	random := ctx.AddFile(0444, 0, "random", "root", root)
	funcs[random.Path] = func(ctx *go9p.ReadContext) {
		// Just grab ctx.Count random bytes and send it
		// to the client.
		data := make([]byte, ctx.Count)
		rand.Reader.Read(data)
		ctx.Respond(data)
	}

	events := ctx.AddFile(0444, 0, "events", "root", root)
	eventFile = events
	funcs[events.Path] = func(ctx *go9p.ReadContext) {
		out := go9p.SliceForRead(ctx, eventData)
		ctx.Respond(out)
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
		Remove: nil,
		Setup:  Setup}
	fmt.Println("Starting server...")

	listener, error := net.Listen("tcp", "0.0.0.0:9999")
	if error != nil {
		fmt.Println("Failed to listen: ", error)
		return
	}

	srv.Serve(listener)
}