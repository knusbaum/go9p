package go9p

import (
	"fmt"
	"net"
)

type Mode uint8
// Open mode file constants
const (
	Oread  = 0
	Owrite = 1
	Ordwr  = 2
	Oexec  = 3
	None   = 4
	Otrunc = 0x10
)

type fidInfo struct {
	path       string
	openMode   Mode
	openOffset uint64
}

func (info *fidInfo) String() string {
	return fmt.Sprintf("[path: %s, openMode: %d, openOffset: %d]",
		info.path, info.openMode, info.openOffset)
}

type connection struct {
	Conn  net.Conn
	fids  map[uint32]*fidInfo
	dirContents map[uint32][]byte // Fid -> serialized directory contents
	readCalled map[uint32]bool
	uname string
	// Auth stuff
	Afid int64  // fids are actually uint32, but I want to signal invalid fid with negative values
	authenticated bool
	iauthch chan []byte
	oauthch chan []byte
}

func (conn *connection) getFidOpenmode(fid uint32) Mode {
	// This can blow up, but if it does, the code is wrong.
	// Only call this method on valid fids.
	info := conn.fids[fid]
	return info.openMode
}

func (conn *connection) setFidOpenmode(fid uint32, openmode Mode) {
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

func (conn *connection) setDirContents(fid uint32, data []byte) {
	if conn.dirContents == nil {
		conn.dirContents = make(map[uint32][]byte)
	}

	conn.dirContents[fid] = data
}

func (conn *connection) getReadCalled() map[uint32]bool {
	if conn.readCalled == nil {
		conn.readCalled = make(map[uint32]bool, 0)
	}
	return conn.readCalled
}

func (conn *connection) accept(l net.Listener) error {
	accepted, err := l.Accept()
	if err != nil {
		return err
	}
	conn.Conn = accepted
	conn.fids = make(map[uint32]*fidInfo, 0)
	conn.Afid = -1
	conn.authenticated = false
	return nil
}
