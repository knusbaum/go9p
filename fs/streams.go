package fs

import (
	"fmt"
	"sync"
	"time"
	"os"
	"io"

	"github.com/knusbaum/go9p/proto"
)

type StreamReader interface {
	Read(p []byte) (n int, err error)
	Close()
}

type chanReader struct {
	read   chan []byte
	unread []byte
	live   bool
}

func (r *chanReader) Read(p []byte) (n int, err error) {
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

func (r *chanReader) Close() {
	r.live = false
	close(r.read)
}

type Stream interface {
	AddReader() StreamReader
	RemoveReader(r StreamReader)
	Write(p []byte) (n int, err error)
	Close() error
}

type baseStream struct {
	readers []*chanReader
	bufflen int
	closed  bool
	sync.Mutex
}

func (s *baseStream) AddReader() StreamReader {
	s.Lock()
	defer s.Unlock()
	reader := &chanReader{
		read: make(chan []byte, s.bufflen),
		live: true,
	}
	if s.closed {
		reader.Close()
	} else {
		s.readers = append(s.readers, reader)
	}
	return reader
}

func (s *baseStream) RemoveReader(r StreamReader) {
	s.Lock()
	defer s.Unlock()
	//fmt.Printf("RemoveReader(%p)\n", r)
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
	r.Close()
}

func (s *baseStream) Close() error {
	s.Lock()
	defer s.Unlock()
	//fmt.Printf("Closing stream %p (%d readers)\n", s, len(s.readers))
	for _, reader := range s.readers {
		//fmt.Printf("Closing reader %p\n", reader)
		reader.Close()
	}
	s.readers = nil
	s.closed = true
	return nil
}

type AsyncStream struct {
	baseStream
}

func NewAsyncStream(buffer int) *AsyncStream {
	return &AsyncStream{
		baseStream{bufflen: buffer},
	}
}

func (s *AsyncStream) Write(p []byte) (n int, err error) {
	s.Lock()
	defer s.Unlock()
	k := 0
	t := time.NewTimer(10 * time.Millisecond)
	for i, reader := range s.readers {
		if !t.Stop() {
			<-t.C
		}
		t.Reset(50 * time.Millisecond)
		cp := make([]byte, len(p))
		copy(cp, p)
		select {
		case reader.read <- cp:
			if i != k {
				s.readers[k] = reader
			}
			k++
		case <-t.C:
			// Writing to writer Timed out.
			reader.Close()
		}
	}
	s.readers = s.readers[:k]
	return len(p), nil
}

type BlockingStream struct {
	baseStream
	waitForReaders bool
	writeLock      sync.Mutex
}

func NewBlockingStream(buffer int, waitForReaders bool) *BlockingStream {
	return &BlockingStream{
		baseStream: baseStream{bufflen: buffer},
		waitForReaders: waitForReaders,
	}
}

func (s *BlockingStream) Write(p []byte) (n int, err error) {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()
	laggers := s.tryWrite(s.readers, p)
	for len(laggers) > 0 {
		//fmt.Printf("LAGGERS: %d\n", len(laggers))
		time.Sleep(10 * time.Millisecond)
		//fmt.Printf("LAGGERS: %d\n", len(laggers))
		laggers = s.tryWrite(laggers, p)
	}
	return len(p), nil
}

func (s *BlockingStream) tryWrite(readers []*chanReader, p []byte) []*chanReader {
	s.Lock()
	defer s.Unlock()
	var laggers []*chanReader
	for _, reader := range readers {
		if !reader.live {
			continue
		}
		cp := make([]byte, len(p))
		copy(cp, p)
		select {
		case reader.read <- cp:
		default:
			laggers = append(laggers, reader)
		}
	}
	return laggers
}

type SkippingStream struct {
	baseStream
}

func NewSkippingStream(buffer int) *SkippingStream {
	return &SkippingStream{
		baseStream{bufflen: buffer},
	}
}

func (s *SkippingStream) Write(p []byte) (n int, err error) {
	s.Lock()
	defer s.Unlock()
	t := time.NewTimer(50 * time.Millisecond)
	for _, reader := range s.readers {
		if !t.Stop() {
			<-t.C
		}
		t.Reset(50 * time.Millisecond)
		cp := make([]byte, len(p))
		copy(cp, p)
		select {
		case reader.read <- cp:
		case <-t.C:
			fmt.Println("Skipping")
			// Timed out. Skip this message.
		}
	}
	return len(p), nil
}

type StreamFile struct {
	*BaseFile
	s         Stream
	fidReader map[uint64]StreamReader
}

func NewStreamFile(stat *proto.Stat, s Stream) *StreamFile {
	return &StreamFile{
		BaseFile:  NewBaseFile(stat),
		s:         s,
		fidReader: make(map[uint64]StreamReader),
	}
}

func (f *StreamFile) Open(fid uint64, omode proto.Mode) error {
	if omode == proto.Oread ||
		omode == proto.Ordwr ||
		omode == proto.Oexec {
		f.fidReader[fid] = f.s.AddReader()
	}
	return nil
}

func (f *StreamFile) Read(fid uint64, offset uint64, count uint64) ([]byte, error) {
	bs := make([]byte, count)
	r, ok := f.fidReader[fid]
	if !ok {
		// This really shouldn't happen.
		return nil, fmt.Errorf("Failed to read stream. Not opened for read.")
	}
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
	r, ok := f.fidReader[fid]
	if ok {
		f.s.RemoveReader(r)
		delete(f.fidReader, fid)
	}
	return nil
}


type fileReader struct {
	f *os.File
	signal chan struct{}
	live   bool
	t *time.Timer
}

func (r *fileReader) Read(p []byte) (n int, err error) {
	if !r.live {
		return 0, io.EOF
	}

	for {
		n, err = r.f.Read(p)
		if err == nil || err != io.EOF {
			return
		}

		if !r.t.Stop() {
			// Need to do this for some reason.
			select {
			case <-r.t.C:
			default:
			}
		}
		r.t.Reset(500 * time.Millisecond)
		select {
		case _, ok := <-r.signal:
			if !ok {
				r.Close()
				return 0, io.EOF
			}
		case <-r.t.C:
		}
	}
}

func (r *fileReader) Close() {
	if r.f != nil {
		r.f.Close()
		r.f = nil
	}
	r.live = false
}

type SavedStream struct {
	f *os.File
	path string
	readers []*fileReader
	closed bool
	sync.Mutex
}

func NewSavedStream(path string) (*SavedStream, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0660)
	if err != nil {
		return nil, err
	}
	return &SavedStream{
		f: f, 
		path: path,
		closed: false,
	}, nil
}

func (s *SavedStream) AddReader() StreamReader {
	s.Lock()
	defer s.Unlock()
	f, err := os.Open(s.path)
	if err != nil {
		return &fileReader{
			f: nil,
			signal: make(chan struct{}, 0),
			live: false,
		}
	}

	reader := &fileReader{
		f: f,
		signal: make(chan struct{}, 1),
		live: true,
		t: time.NewTimer(500 * time.Millisecond),
	}

	if s.closed {
		close(reader.signal)
	} else {
		s.readers = append(s.readers, reader)
	}
	return reader
}

func (s *SavedStream) RemoveReader(r StreamReader) {
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
	r.Close()
}

func (s *SavedStream) Write(p []byte) (n int, err error) {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return 0, io.EOF // TODO: Should this be EOF?
	}
	n, err = s.f.Write(p)
	if err != nil {
		return
	}
	k := 0
	for i, reader := range s.readers {
		if reader.live {
			select {
			case reader.signal<- struct{}{}:
			default:
			}
			if i != k {
				s.readers[k] = reader
			}
			k++
		}
	}
	s.readers = s.readers[:k]
	return
}

func (s *SavedStream) Close() error {
	s.Lock()
	defer s.Unlock()
	for _, reader := range s.readers {
		close(reader.signal)
	}
	s.closed = true
	s.readers = nil
	s.f.Close()
	s.f = nil
	return nil
}