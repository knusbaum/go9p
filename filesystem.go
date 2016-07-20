package go9p

import (
	"fmt"
)

type File struct {
	Path string
	Stat Stat
	subfiles []*File
	Parent *File

	Contents []byte
}

type Filesystem struct {
	files map[string]*File /* path -> *File */
	currUid uint64
}

func (fs *Filesystem) DumpFiles() {
	for k, v := range fs.files {
		fmt.Println("Path: %s, File.path: %s, len(Contents): %d, cap(Contents): %d\n",
			k, v.Path, len(v.Contents), cap(v.Contents))
	}
}

func InitializeFs() Filesystem {
	fs := Filesystem{}
	fs.files = make(map[string]*File, 1)
	return fs
}

func (fs *Filesystem) AllocQid(qtype uint8) Qid {
	uid := fs.currUid
	fs.currUid = fs.currUid + 1
	return Qid{
		Qtype: qtype,
		Vers: 0,
		Uid: uid}
}

func (fs *Filesystem) AddFile(path string, stat Stat, parent *File) *File {
	file := &File{path, stat, make([]*File, 0), parent, make([]byte, 0)}
	fs.files[path] = file
	if parent != nil {
		parent.subfiles = append(parent.subfiles, file)
	}
	return file
}

func (fs *Filesystem) RemoveFile(file *File) {
	if file.Parent != nil {
		// Need to remove this file from its parent's list.
		parent := file.Parent
		for i, f := range parent.subfiles {
			if f == file {
				parent.subfiles = append(parent.subfiles[:i], parent.subfiles[i+1:]...)
			}
		}
	}

	delete(fs.files, file.Path)
	
}

func (fs *Filesystem) FileForPath(path string) *File {
	return fs.files[path]
}

type Qid struct {
	Qtype uint8
	Vers uint32
	Uid uint64
}

func (qid *Qid) String() string {
	return fmt.Sprintf("qtype: 0x%X, version: %d, uid: %d",
		qid.Qtype, qid.Vers, qid.Uid)
}

func (qid *Qid) Parse(buff []byte) ([]byte, error) {
	if len(buff) == 0 {
		return nil, &ParseError{"can't parse. Reached end of buffer."}
	}
	qid.Qtype = buff[0]
	qid.Vers, buff = FromLittleE32(buff[1:])
	qid.Uid, buff = FromLittleE64(buff)
	return buff, nil
}

func (qid *Qid) Compose() []byte {
	buff := make([]byte, 13)
	buffer := buff

	buffer[0] = qid.Qtype; buffer = buffer[1:]
	buffer = ToLittleE32(qid.Vers, buffer)
	buffer = ToLittleE64(qid.Uid, buffer)

	return buff
}
