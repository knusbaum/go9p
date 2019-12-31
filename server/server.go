package server

import (
	"bufio"
	"fmt"
	"io"
	"net"

	"github.com/knusbaum/go9p/fs"
	"github.com/knusbaum/go9p/proto"
)

func handleConnection(nc net.Conn, fs *fs.FS) {
	defer nc.Close()
	read := bufio.NewReader(nc)
	err := handleIO(read, nc, fs)
	if err != nil {
		fmt.Printf("%v\n", err)
	}
}

func handleIO(r io.Reader, w io.Writer, fs *fs.FS) error {
	c := conn{fs: fs, uname: "none", fids: make(map[uint32]*fidInfo)}
	for {
		call, err := proto.ParseCall(r)
		if err != nil {
			return err
		}
		fmt.Printf("=in=> %v\n", call)
		resp, err := c.handleCall(call)
		if err != nil {
			return err
		}
		fmt.Printf("<=out= %v\n", resp)
		_, err = w.Write(resp.Compose())
		if err != nil {
			return err
		}
	}
	return nil
}

func Serve(addr string, fs *fs.FS) error {
	l, err := net.Listen("tcp", "0.0.0.0:9999")
	if err != nil {
		return err
	}
	for {
		a, err := l.Accept()
		if err != nil {
			return err
		}
		go handleConnection(a, fs)
	}
}

func PostSrv(name string, fs *fs.FS) error {
	f, err := postfd(name)
	if err != nil {
		return err
	}
	defer f.Close()
	err = handleIO(f, f, fs)
	return err
}
