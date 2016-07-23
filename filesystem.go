package go9p

import (
	"fmt"
)

// File - Represents a file in the filesystem.
// Path - The full path of the file in the filesystem.
// Stat - The Stat struct associated with the file.
// Parent - A pointer to the file's parent directory.
type File struct {
	Path     string
	Stat     Stat
	Parent   *File
	subfiles []*File
}

func (f *File)IsDirectory() bool {
	return f.Stat.Mode & (1 << 31) != 0
}

func (f *File)ListSubfiles() []*File {
	ret := make([]*File, len(f.subfiles))
	copy(ret, f.subfiles)
	return ret
}

func (f *File)composeSubfiles() []byte{
	contents := make([]byte, 0)
	for _, sf := range f.subfiles {
		contents = append(contents, sf.Stat.Compose()...)
	}
	return contents
}

type update struct {
	originalCtx *Ctx
	fn func(*UpdateContext)
}

type filesystem struct {
	files   map[string]*File /* path -> *File */
	currUid uint64
	updateChan chan update
}

func (fs *filesystem) dumpFiles() {
	for k, v := range fs.files {
		fmt.Println("Path: %s, File.path: %s\n",
			k, v.Path)
	}
}

func initializeFs() filesystem {
	fs := filesystem{}
	fs.files = make(map[string]*File, 1)
	fs.updateChan = make(chan update)
	return fs
}

func (fs *filesystem) allocQid(qtype uint8) Qid {
	uid := fs.currUid
	fs.currUid = fs.currUid + 1
	return Qid{
		Qtype: qtype,
		Vers:  0,
		Uid:   uid}
}

func (fs *filesystem) addFile(path string, stat Stat, parent *File) *File {
	file := &File{path, stat, parent, make([]*File, 0)}
	fs.files[path] = file
	if parent != nil {
		parent.subfiles = append(parent.subfiles, file)
	}
	return file
}

func (fs *filesystem) removeFile(file *File) {
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

func (fs *filesystem) fileForPath(path string) *File {
	return fs.files[path]
}

// Qid - Qids are unique ids for files. Qtype should be the upper
// 8 bits of the file's permissions (Stat.Mode)
type Qid struct {
	Qtype uint8
	Vers  uint32
	Uid   uint64
}

func (qid *Qid) String() string {
	return fmt.Sprintf("qtype: 0x%X, version: %d, uid: %d",
		qid.Qtype, qid.Vers, qid.Uid)
}

// Parse - Parse a Qid from a slice of a 9P2000 stream
func (qid *Qid) Parse(buff []byte) ([]byte, error) {
	if len(buff) == 0 {
		return nil, &ParseError{"can't parse. Reached end of buffer."}
	}
	qid.Qtype = buff[0]
	qid.Vers, buff = fromLittleE32(buff[1:])
	qid.Uid, buff = fromLittleE64(buff)
	return buff, nil
}

// Compose - Returns a slice of the Qid serialized to be
// written out on a 9P2000 stream.
func (qid *Qid) Compose() []byte {
	buff := make([]byte, 13)
	buffer := buff

	buffer[0] = qid.Qtype
	buffer = buffer[1:]
	buffer = toLittleE32(qid.Vers, buffer)
	buffer = toLittleE64(qid.Uid, buffer)

	return buff
}
