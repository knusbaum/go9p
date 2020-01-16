package main

import (
	"github.com/knusbaum/go9p/fs"
	"github.com/knusbaum/go9p"
)

func main() {
	fs := fs.NewFS("glenda", "glenda", 0777,
		fs.WithCreateFile(fs.CreateStaticFile),
		fs.WithCreateDir(fs.CreateStaticDir),
		fs.WithRemoveFile(fs.RMFile),
	)
	// Listen on port 9999
	go9p.Serve("0.0.0.0:9999", fs.Server())
}
