package main

import (
	"log"

	"github.com/knusbaum/go9p2/server"
	"github.com/knusbaum/go9p2/fs"
)

func main() {
	fs := fs.NewFS("glenda", "glenda", 0777,
		fs.WithCreateFile(fs.CreateStaticFile),
		fs.WithCreateDir(fs.CreateStaticDir),
		fs.WithRemoveFile(fs.RMFile),
	)
	server.Serve("0.0.0.0:9999", fs)
}