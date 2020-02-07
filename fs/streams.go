package fs

import (
	"fmt"
	"sync"
	"time"

	"github.com/knusbaum/go9p/proto"
)

type StreamReader struct {
	read   chan []byte
	unread []byte
}

func (r *StreamReader) Read(p []byte) (n int, err error) {
	for len(p) > 0 {
		if len(r.unread) == 0 {
			if n > 0 {
				select {
				case bs, ok := <-r.read:
					if !ok {
						return
					}
					r.unread = bs
				default:
					return
				}
			} else {
				bs, ok := <-r.read
				if !ok {
					return 0, fmt.Errorf("End Of File.") // TODO: replace with real io.EOF
				}
				r.unread = bs
			}
		}
		newn := copy(p, r.unread)
		r.unread = r.unread[newn:]
		p = p[newn:]
		n += newn
		if len(p) == 0 {
			return
		}
	}
	return
}

type Stream struct {
	readers []*StreamReader
	bufflen int
	closed  bool
	blockOnReader bool
	sync.Mutex
}

func NewStream(buffer int, blockOnReader bool) *Stream {
	return &Stream{
		bufflen: buffer,
		blockOnReader: blockOnReader,
	}
}

func (s *Stream) AddReader() *StreamReader {
	s.Lock()
	defer s.Unlock()
	reader := &StreamReader{
		read: make(chan []byte, s.bufflen),
	}
	if s.closed {
		close(reader.read)
	} else {
		s.readers = append(s.readers, reader)
	}
	return reader
}

func (s *Stream) RemoveReader(r *StreamReader) {
	s.Lock()
	defer s.Unlock()
	k := 0
	for i, reader := range s.readers {
		if r != reader {
			if i != k {
				s.readers[k] = reader
			}
			k++
		}
	}
	s.readers = s.readers[:k]
}

func (s *Stream) Write(p []byte) (n int, err error) {
	s.Lock()
	defer s.Unlock()
	k := 0
	for i, reader := range s.readers {
		cp := make([]byte, len(p))
		copy(cp, p)
		if s.blockOnReader {
			reader.read <- cp
		} else {
			select {
			case reader.read <- cp:
				if i != k {
					s.readers[k] = reader
				}
				k++
			case <-time.After(100 * time.Millisecond):
				// Writing to writer Timed out.
				close(reader.read)
			}
		}
	}
	s.readers = s.readers[:k]
	return len(p), nil
}

func (s *Stream) Close() error {
	s.Lock()
	defer s.Unlock()
	for _, reader := range s.readers {
		close(reader.read)
	}
	s.readers = nil
	s.closed = true
	return nil
}

type StreamFile struct {
	*BaseFile
	s         *Stream
	fidReader map[uint64]*StreamReader
}

func NewStreamFile(stat *proto.Stat, s *Stream) *StreamFile {
	return &StreamFile{
		BaseFile:  NewBaseFile(stat),
		s:         s,
		fidReader: make(map[uint64]*StreamReader),
	}
}

func (f *StreamFile) Open(fid uint64, omode proto.Mode) error {
	f.fidReader[fid] = f.s.AddReader()
	return nil
}

func (f *StreamFile) Read(fid uint64, offset uint64, count uint64) ([]byte, error) {
	bs := make([]byte, count)
	r := f.fidReader[fid]
	n, err := r.Read(bs)
	if err != nil {
		return nil, err
	}
	bs = bs[:n]
	return bs, nil
}

func (f *StreamFile) Write(fid uint64, offset uint64, data []byte) (uint32, error) {
	n, err := f.s.Write(data)
	return uint32(n), err
}

func (f *StreamFile) Close(fid uint64) error {
	r := f.fidReader[fid]
	f.s.RemoveReader(r)
	return nil
}

