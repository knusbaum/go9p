package main

import (
	"log"
	"time"
	"fmt"

	"github.com/knusbaum/go9p"
	"github.com/knusbaum/go9p/fs"
)

func main() {
	sfs := fs.NewFS("glenda", "glenda", 0777)

	stream, err := fs.NewSavedStream("/tmp/savedStream")
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		i := 0
		for {
			time.Sleep(5 * time.Second)
			log.Printf("Writing %d\n", i)
			stream.Write([]byte(fmt.Sprintf("%d\n", i)))
			i += 1
		}
	}()

	sfs.Root.AddChild(fs.NewStreamFile(
		sfs.NewStat("savedStream", "glenda", "glenda", 0666),
		stream,
	))

	log.Println("Starting server.")
	// Listen on port 9999
	go9p.Serve("0.0.0.0:9999", sfs.Server())
}
