package fs

import (
	"errors"
	"fmt"

	"github.com/knusbaum/go9p/proto"
)

type StreamFile struct {
	*BaseFile
	s         Stream
	fidReader map[uint64]StreamReader
}

type BiDiStreamFile struct {
	*BaseFile
	s         BiDiStream
	fidReader map[uint64]StreamReadWriter
}

func NewStreamFile(stat *proto.Stat, s Stream) File {
	if bidi, ok := s.(BiDiStream); ok {
		return &BiDiStreamFile{
			BaseFile:  NewBaseFile(stat),
			s:         bidi,
			fidReader: make(map[uint64]StreamReadWriter),
		}
	}
	return &StreamFile{
		BaseFile:  NewBaseFile(stat),
		s:         s,
		fidReader: make(map[uint64]StreamReader),
	}
}

func (f *StreamFile) Stat() proto.Stat {
	stat := f.fStat
	stat.Length = f.s.length()
	return stat
}

func (f *StreamFile) Open(fid uint64, omode proto.Mode) error {
	if omode == proto.Owrite ||
		omode == proto.Ordwr {
		return errors.New("Cannot open this stream for writing.")
	}
	f.fidReader[fid] = f.s.AddReader()
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
	return 0, errors.New("Cannot write to this stream.")
}

func (f *StreamFile) Close(fid uint64) error {
	r, ok := f.fidReader[fid]
	if ok {
		f.s.RemoveReader(r)
		delete(f.fidReader, fid)
	}
	return nil
}

func (f *BiDiStreamFile) Stat() proto.Stat {
	stat := f.fStat
	stat.Length = f.s.length()
	return stat
}

func (f *BiDiStreamFile) Open(fid uint64, omode proto.Mode) error {
	f.fidReader[fid] = f.s.AddReadWriter()
	return nil
}

func (f *BiDiStreamFile) Read(fid uint64, offset uint64, count uint64) ([]byte, error) {
	bs := make([]byte, count)
	r, ok := f.fidReader[fid]
	if !ok {
		// This really shouldn't happen.
		return nil, fmt.Errorf("Failed to read stream. Server error.")
	}
	n, err := r.Read(bs)
	if err != nil {
		return nil, err
	}
	bs = bs[:n]
	return bs, nil
}

func (f *BiDiStreamFile) Write(fid uint64, offset uint64, data []byte) (uint32, error) {
	r, ok := f.fidReader[fid]
	if !ok {
		// This really shouldn't happen.
		return 0, fmt.Errorf("Failed to write stream. Server error.")
	}
	n, err := r.Write(data)
	return uint32(n), err
}

func (f *BiDiStreamFile) Close(fid uint64) error {
	r, ok := f.fidReader[fid]
	if ok {
		f.s.RemoveReader(r)
		delete(f.fidReader, fid)
	}
	return nil
}
