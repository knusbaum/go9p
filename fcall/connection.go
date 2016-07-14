package fcall

import (
	"net"
	"fmt"
)

const (
	Oread = 0
	Owrite = 1
	Ordwr = 2
	Oexec = 3
	None = 4
)

type FidInfo struct {
	path string
	openMode uint8
	openOffset uint64
}

func (info *FidInfo) String() string {
	return fmt.Sprintf("[path: %s, openMode: %d, openOffset: %d]",
		info.path, info.openMode, info.openOffset)
}

type Connection struct {
	Conn net.Conn
	fids map[uint32]*FidInfo
	uname string
}

func (conn *Connection) GetFidOpenmode(fid uint32) uint8 {
	// This can blow up, but if it does, the code is wrong.
	// Only call this method on valid fids.
	info := conn.fids[fid]
	return info.openMode
}

func(conn *Connection) SetFidOpenmode(fid uint32, openmode uint8) {
	fmt.Printf("Setting openmode to: %d for fid: %d\n", openmode, fid)
	info := conn.fids[fid]
	fmt.Printf("Info is: %s\n", info)
	if info != nil {
		info.openMode = openmode
	}
}

func (conn *Connection) GetFidOpenoffset(fid uint32) uint64 {
	info := conn.fids[fid]
	return info.openOffset
}

func (conn *Connection) SetFidOpenoffset(fid uint32, openoffset uint64) {
	info := conn.fids[fid]
	if info != nil {
		info.openOffset = openoffset
	}
}

func (conn *Connection) PathForFid(fid uint32) string {
	info := conn.fids[fid]
	if(info == nil) {
		return ""
	}
	return info.path
}

func (conn *Connection) SetFidPath(fid uint32, path string) {
	fmt.Printf("Setting path to: [%s] for fid: %d\n", path, fid)
	if conn.fids == nil {
		conn.fids = make(map[uint32]*FidInfo, 1)
	}
	conn.fids[fid] = &FidInfo{path, None, 0}
}

func (conn *Connection) SetUname(uname string) {
	conn.uname = uname
}

func (conn *Connection) GetUname() string {
	return conn.uname
}

func (conn *Connection) Accept(l net.Listener) error {
	accepted, err := l.Accept()
	if err != nil {
		return err
	}
	conn.Conn = accepted
	conn.fids = make(map[uint32]*FidInfo, 0)
	return nil
}
