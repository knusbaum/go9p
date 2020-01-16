package go9p

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"reflect"

	"github.com/knusbaum/go9p/proto"
)

// The Srv interface is used to handle 9p2000 messages.
// Each function handles a specific type of message, and
// should return a response. If some expected error occurs,
// for example a TOpen message for a file with the wrong
// permissions, a proto.TError message should be returned
// rather than a go error. Returning a go error indicates that
// something has gone wrong with the server, and when used with
// Serve and PostSrv, will cause the connection to be terminated
// or the file descriptor to be closed.
type Srv interface {
	NewConn() Conn
	Version(Conn, *proto.TRVersion) (proto.FCall, error)
	Auth(Conn, *proto.TAuth) (proto.FCall, error)
	Attach(Conn, *proto.TAttach) (proto.FCall, error)
	Flush(Conn, *proto.TFlush) (proto.FCall, error)
	Walk(Conn, *proto.TWalk) (proto.FCall, error)
	Open(Conn, *proto.TOpen) (proto.FCall, error)
	Create(Conn, *proto.TCreate) (proto.FCall, error)
	Read(Conn, *proto.TRead) (proto.FCall, error)
	Write(Conn, *proto.TWrite) (proto.FCall, error)
	Clunk(Conn, *proto.TClunk) (proto.FCall, error)
	Remove(Conn, *proto.TRemove) (proto.FCall, error)
	Stat(Conn, *proto.TStat) (proto.FCall, error)
	Wstat(Conn, *proto.TWstat) (proto.FCall, error)
}

// Conn represents an individual connection to a 9p server.
// In the case of a server listening on a network, there
// may be many clients connected to a given server at once.
type Conn interface {
	Uname() string
}

func handleConnection(nc net.Conn, srv Srv) {
	defer nc.Close()
	read := bufio.NewReader(nc)
	err := handleIO(read, nc, srv)
	if err != nil {
		fmt.Printf("%v\n", err)
	}
}

func handleIO(r io.Reader, w io.Writer, srv Srv) error {
	conn := srv.NewConn()
	for {
		call, err := proto.ParseCall(r)
		if err != nil {
			return err
		}
		fmt.Printf("=in=> %v\n", call)
		resp, err := handleCall(call, srv, conn)
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

func handleCall(call proto.FCall, srv Srv, conn Conn) (proto.FCall, error) {
	switch call.(type) {
	case *proto.TRVersion:
		return srv.Version(conn, call.(*proto.TRVersion))
	case *proto.TAuth:
		return srv.Auth(conn, call.(*proto.TAuth))
	case *proto.TAttach:
		return srv.Attach(conn, call.(*proto.TAttach))
	case *proto.TFlush:
		return srv.Flush(conn, call.(*proto.TFlush))
	case *proto.TWalk:
		return srv.Walk(conn, call.(*proto.TWalk))
	case *proto.TOpen:
		return srv.Open(conn, call.(*proto.TOpen))
	case *proto.TCreate:
		return srv.Create(conn, call.(*proto.TCreate))
	case *proto.TRead:
		return srv.Read(conn, call.(*proto.TRead))
	case *proto.TWrite:
		return srv.Write(conn, call.(*proto.TWrite))
	case *proto.TClunk:
		return srv.Clunk(conn, call.(*proto.TClunk))
	case *proto.TRemove:
		return srv.Remove(conn, call.(*proto.TRemove))
	case *proto.TStat:
		return srv.Stat(conn, call.(*proto.TStat))
	case *proto.TWstat:
		return srv.Wstat(conn, call.(*proto.TWstat))
	default:
		return nil, fmt.Errorf("Invalid call: %s", reflect.TypeOf(call))
	}
}

// Serve serves srv on the given address, addr.
func Serve(addr string, srv Srv) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	for {
		a, err := l.Accept()
		if err != nil {
			return err
		}
		go handleConnection(a, srv)
	}
}

// PostSrv serves srv, from a file descriptor named name.
// The fd is posted and can subsequently be mounted. On Unix, the
// descriptor is posted under in the current namespace, which is
// determined by 9fans.net/go/plan9/client Namespace. On Plan9 it
// is posted in the usual place, /srv.
func PostSrv(name string, srv Srv) error {
	f, err := postfd(name)
	if err != nil {
		return err
	}
	defer f.Close()
	err = handleIO(f, f, srv)
	return err
}
