package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/knusbaum/go9p"
	"github.com/knusbaum/go9p/fs"
	"github.com/knusbaum/go9p/fs/real"
)

var exportFS fs.FS

func main() {
	directory := flag.String("dir", ".", "The directory that will be exported")
	address := flag.String("address", "localhost:9000", "The address on which to listed for incoming 9p connections")
	srv := flag.String("srv", "", "If specified, exportfs will listen on a unix socket with this service name in the current namespace (see p9p namespace(1)) rather than listening on tcp")
	verbose := flag.Bool("v", false, "Makes the 9p protocol verbose, printing all incoming and outgoing messages.")
	stdio := flag.Bool("s", false, "Serve 9p over standard in and standard out.")
	noperm := flag.Bool("noperm", false, "Ignore permissions enforcement. Any attached user will have the same filesystem permissions as the user running export9p.")
	flag.Parse()

	if flag.NArg() > 0 {
		log.Printf("Extraneous arguments.")
		flag.Usage()
		os.Exit(1)
	}

	go9p.Verbose = *verbose

	dir, err := filepath.Abs(*directory)
	if err != nil {
		log.Printf("Error: %s", dir)
		flag.Usage()
		os.Exit(1)
	}

	exportFS.Root = &real.Dir{Path: dir}
	fs.WithCreateFile(real.CreateFile)(&exportFS)
	fs.WithCreateDir(real.CreateDir)(&exportFS)
	fs.WithRemoveFile(real.Remove)(&exportFS)
	if *noperm {
		fs.IgnorePermissions()(&exportFS)
	}
	if *stdio {
		if *verbose {
			log.Printf("Serving %s on standard input/output", dir)
		}
		err = go9p.ServeReadWriter(os.Stdin, os.Stdout, exportFS.Server())
	} else if *srv != "" {
		if *verbose {
			log.Printf("Serving %s as service %s", dir, *srv)
		}
		err = go9p.PostSrv(*srv, exportFS.Server())
	} else {
		if *verbose {
			log.Printf("Serving %s on %s", dir, *address)
		}
		err = go9p.Serve(*address, exportFS.Server())
	}
	if err != nil {
		log.Fatal(err)
	}
}
