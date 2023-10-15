package union

import (
	"io"
	"testing"

	"github.com/knusbaum/go9p"
	"github.com/knusbaum/go9p/client"
	"github.com/knusbaum/go9p/fs"
	"github.com/knusbaum/go9p/proto"
)

type TwoPipe struct {
	*io.PipeReader
	*io.PipeWriter
}

func (t *TwoPipe) Close() error {
	t.PipeReader.Close()
	t.PipeWriter.Close()
	return nil
}

func TestSingleRootMount(t *testing.T) {
	ufs := NewUnionFS()

	rootfs, rootfsdir := fs.NewFS("glenda", "glenda", 0555)
	bindir := fs.NewStaticDir(rootfs.NewStat("bin", "glenda", "glenda", 0555))
	bindir.AddChild(fs.NewStaticFile(rootfs.NewStat("ls", "glenda", "glenda", 0444), []byte("Binary data\n")))
	rootfsdir.AddChild(bindir)
	p1r, p1w := io.Pipe()
	p2r, p2w := io.Pipe()
	go go9p.ServeReadWriter(p1r, p2w, rootfs.Server())

	rootc, err := client.NewClient(&TwoPipe{p2r, p1w}, "glenda", "glenda")
	if err != nil {
		t.Fatalf("%s", err)
	}

	err = Mount(ufs, rootc, "/", REPLACE, false)
	if err != nil {
		t.Fatalf("%s", err)
	}

	bin, ok := ufs.Root.Children()["bin"]
	if !ok {
		t.Fatalf("/bin not found")
	}

	ls, ok := bin.(fs.Dir).Children()["ls"]
	if !ok {
		t.Fatalf("/bin/ls not found")
	}

	lsf := ls.(fs.File)
	lsf.Open(1234, proto.Oread)
	lsc, err := lsf.Read(1234, 0, 1024)
	if err != nil {
		t.Fatalf("%s", err)
	}

	if string(lsc) != "Binary data\n" {
		t.Fatalf("Contents of ls doesn't match: '%s' and '%s'", string(lsc), "Binary data\n")
	}
}

func TestUnionBinMount(t *testing.T) {
	ufs := NewUnionFS()

	// This is the root ('/') with directories /bin (with ls) and /usr.
	rootfs, rootfsdir := fs.NewFS("glenda", "glenda", 0555)
	bindir := fs.NewStaticDir(rootfs.NewStat("bin", "glenda", "glenda", 0555))
	bindir.AddChild(fs.NewStaticFile(rootfs.NewStat("ls", "glenda", "glenda", 0444), []byte("Binary data\n")))
	rootfsdir.AddChild(bindir)
	rootfsdir.AddChild(fs.NewStaticDir(rootfs.NewStat("usr", "glenda", "glenda", 0555)))
	p1r, p1w := io.Pipe()
	p2r, p2w := io.Pipe()
	rootpipe := &TwoPipe{p2r, p1w}
	go go9p.ServeReadWriter(p1r, p2w, rootfs.Server())

	// This is a /usr filesystem
	usrfs, usrfsdir := fs.NewFS("glenda", "glenda", 0555)
	usrbindir := fs.NewStaticDir(usrfs.NewStat("bin", "glenda", "glenda", 0555))
	usrbindir.AddChild(fs.NewStaticFile(usrfs.NewStat("cat", "glenda", "glenda", 0444), []byte("More binary data\n")))
	usrfsdir.AddChild(usrbindir)
	p1r, p1w = io.Pipe()
	p2r, p2w = io.Pipe()
	usrpipe := &TwoPipe{p2r, p1w}
	go go9p.ServeReadWriter(p1r, p2w, usrfs.Server())

	// Mount /
	rootc, err := client.NewClient(rootpipe, "glenda", "glenda")
	if err != nil {
		t.Fatalf("error connecting to /:%s", err)
	}
	err = Mount(ufs, rootc, "/", REPLACE, false)
	if err != nil {
		t.Fatalf("error mounting /: %s", err)
	}

	// Mount /usr
	usrc, err := client.NewClient(usrpipe, "glenda", "glenda")
	if err != nil {
		t.Fatalf("error connecting to /usr: %s", err)
	}
	err = Mount(ufs, usrc, "/usr", REPLACE, false)
	if err != nil {
		t.Fatalf("error mounting /usr: %s", err)
	}

	usrbin := ufs.Root.(*UnionDir).findDir("/usr/bin")
	if usrbin == nil {
		t.Fatalf("/usr/bin not found")
	}

	cat := usrbin.findFile("cat")
	if cat == nil {
		t.Fatalf("/usr/bin/cat not found")
	}

	// Bind /usr/bin to /bin
	err = Bind(ufs, "/usr/bin", "/bin", AFTER, false)
	if err != nil {
		t.Fatalf("error binding /usr/bin to /bin: %s", err)
	}

	bin := ufs.Root.(*UnionDir).findDir("/bin")
	if bin == nil {
		t.Fatalf("/bin not found")
	}

	binchild := bin.Children()
	if len(binchild) != 2 {
		t.Fatalf("/bin doesn't have the union of /bin and /usr/bin: %v mount table: %+v", bin.Children(), bin.mountTable)
	}

	if ls, ok := binchild["ls"]; !ok {
		t.Fatalf("ls is not found in /bin")
	} else {
		lsf := ls.(*UnionFile)
		lsf.Open(1234, proto.Oread)
		lsc, err := lsf.Read(1234, 0, 1024)
		if err != nil {
			t.Fatalf("Failure to read ls: %s", err)
		}

		if string(lsc) != "Binary data\n" {
			t.Fatalf("Contents of ls doesn't match 'Binary data\n': %s", string(lsc))
		}
	}

	if cat, ok := binchild["cat"]; !ok {
		t.Fatalf("cat is not found in /bin")
	} else {
		catf := cat.(*UnionFile)
		catf.Open(2345, proto.Oread)
		catc, err := catf.Read(2345, 0, 1024)
		if err != nil {
			t.Fatalf("Failure to read cat: %s", err)
		}

		if string(catc) != "More binary data\n" {
			t.Fatalf("Contents of cat doesn't match 'More binary data\n': %s", string(catc))
		}
	}
}
