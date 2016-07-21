package go9p

import (
	"fmt"
	"net"
)

// Open mode file constants
const (
	Oread  = 0
	Owrite = 1
	Ordwr  = 2
	Oexec  = 3
	None   = 4
)

type fidInfo struct {
	path       string
	openMode   uint8
	openOffset uint64
}

func (info *fidInfo) String() string {
	return fmt.Sprintf("[path: %s, openMode: %d, openOffset: %d]",
		info.path, info.openMode, info.openOffset)
}

type connection struct {
	Conn  net.Conn
	fids  map[uint32]*fidInfo
	uname string
}

func (conn *connection) getFidOpenmode(fid uint32) uint8 {
	// This can blow up, but if it does, the code is wrong.
	// Only call this method on valid fids.
	info := conn.fids[fid]
	return info.openMode
}

func (conn *connection) setFidOpenmode(fid uint32, openmode uint8) {
	info := conn.fids[fid]
	if info != nil {
		info.openMode = openmode
	}
}

func (conn *connection) getFidOpenoffset(fid uint32) uint64 {
	info := conn.fids[fid]
	return info.openOffset
}

func (conn *connection) setFidOpenoffset(fid uint32, openoffset uint64) {
	info := conn.fids[fid]
	if info != nil {
		info.openOffset = openoffset
	}
}

func (conn *connection) pathForFid(fid uint32) string {
	info := conn.fids[fid]
	if info == nil {
		return ""
	}
	return info.path
}

func (conn *connection) setFidPath(fid uint32, path string) {
	if conn.fids == nil {
		conn.fids = make(map[uint32]*fidInfo, 1)
	}
	conn.fids[fid] = &fidInfo{path, None, 0}
}

func (conn *connection) accept(l net.Listener) error {
	accepted, err := l.Accept()
	if err != nil {
		return err
	}
	conn.Conn = accepted
	conn.fids = make(map[uint32]*fidInfo, 0)
	return nil
}
