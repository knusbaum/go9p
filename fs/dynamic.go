package fs

import (
	"github.com/knusbaum/go9p/proto"
)

type DynamicFile struct {
	BaseFile
	fidContent map[uint32][]byte
	genContent func() []byte
}

func NewDynamicFile(s *proto.Stat, genContent func() []byte) *DynamicFile {
	return &DynamicFile{
		BaseFile:   BaseFile{fStat: *s},
		fidContent: make(map[uint32][]byte),
		genContent: genContent,
	}
}

func (f *DynamicFile) Open(fid uint32, omode uint8) error {
	f.Lock()
	defer f.Unlock()
	f.fidContent[fid] = f.genContent()
	return nil
}

func (f *DynamicFile) Read(fid uint32, offset uint64, count uint64) ([]byte, error) {
	f.RLock()
	defer f.RUnlock()

	data := f.fidContent[fid]

	flen := uint64(len(data))
	if offset >= flen {
		return []byte{}, nil
	}
	if offset+count > flen {
		count = flen - offset
	}
	return data[offset : offset+count], nil
}

func (f *DynamicFile) Close(fid uint32) error {
	delete(f.fidContent, fid)
	return nil
}

type WrappedFile struct {
	File
	OpenF  func(fid uint32, omode uint8) error
	ReadF  func(fid uint32, offset uint64, count uint64) ([]byte, error)
	WriteF func(fid uint32, offset uint64, data []byte) (uint32, error)
	CloseF func(fid uint32) error
}

func (f *WrappedFile) Open(fid uint32, omode uint8) error {
	if f.OpenF != nil {
		return f.OpenF(fid, omode)
	}
	return f.File.Open(fid, omode)
}

func (f *WrappedFile) Read(fid uint32, offset uint64, count uint64) ([]byte, error) {
	if f.ReadF != nil {
		return f.ReadF(fid, offset, count)
	}
	return f.File.Read(fid, offset, count)
}

func (f *WrappedFile) Write(fid uint32, offset uint64, data []byte) (uint32, error) {
	if f.WriteF != nil {
		return f.WriteF(fid, offset, data)
	}
	return f.File.Write(fid, offset, data)
}

func (f *WrappedFile) Close(fid uint32) error {
	if f.CloseF != nil {
		return f.CloseF(fid)
	}
	return f.File.Close(fid)
}
