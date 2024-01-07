package union

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/knusbaum/go9p"
	"github.com/knusbaum/go9p/client"
	"github.com/knusbaum/go9p/fs"
	"github.com/knusbaum/go9p/proto"
)

type twoPipe struct {
	*io.PipeReader
	*io.PipeWriter
}

func (t *twoPipe) Close() error {
	t.PipeReader.Close()
	t.PipeWriter.Close()
	return nil
}

func findDir(t *testing.T, ud fs.Dir, rel string) fs.Dir {
	rel = strings.TrimPrefix(rel, "/")

	if rel == "" {
		return ud
	}

	parts := strings.Split(rel, "/")
	d := ud
	var n fs.Dir

	for _, part := range parts {
		if d == nil {
			t.Fatalf("%s not found in %s", rel, ud)
			return nil
		}

		children := d.Children()
		child, ok := children[part]
		if !ok {
			t.Fatalf("%s not found in %s", rel, ud)
			return nil
		}

		n, ok = child.(fs.Dir)
		d = n
		if !ok {
			t.Fatalf("%s not found in %s", rel, ud)
			return nil
		}
	}

	return n
}

func findFile(t *testing.T, ud fs.Dir, rel string) fs.File {
	rel = strings.TrimPrefix(rel, "/")

	if rel == "" {
		t.Fatalf("empty relative path")
		return nil
	}

	parts := strings.Split(rel, "/")
	d := ud
	var n fs.FSNode

	for _, part := range parts {
		if d == nil {
			t.Fatalf("%s not found in %s", rel, ud)
			return nil
		}

		children := d.Children()
		child, ok := children[part]
		if !ok {
			t.Fatalf("%s not found in %s", rel, ud)
			return nil
		}

		n = child
		if cd, ok := child.(fs.Dir); ok {
			d = cd
		}
	}

	if uf, ok := n.(fs.File); ok {
		return uf
	} else {
		t.Fatalf("%s not found in %s", rel, ud)
	}

	t.Fatalf("%s not found in %s", rel, ud)
	return nil
}

func mustNewClient(rwc io.ReadWriteCloser) *client.Client {
	c, err := client.NewClient(rwc, "glenda", "glenda")
	if err != nil {
		panic(err)
	}

	return c
}

func mustMount(ufs *fs.FS, c *client.Client, old string, option MountOption, create bool) {
	err := Mount(ufs, c, old, option, create)
	if err != nil {
		panic(err)
	}
}

func mustBind(ufs *fs.FS, path string, old string, option MountOption, create bool) {
	err := Bind(ufs, path, old, option, create)
	if err != nil {
		panic(err)
	}
}

func assertFile(parent fs.Dir, name string, contents string) {
	children := parent.Children()

	if f, ok := children[name]; !ok {
		panic(fmt.Errorf("%s is not found in %s", name, parent))
	} else {
		ff := f.(fs.File)
		err := ff.Open(1234, proto.Oread)
		if err != nil {
			panic(err)
		}
		defer ff.Close(1234)

		fc, err := ff.Read(1234, 0, 1024)
		if err != nil {
			panic(fmt.Errorf("failure to read %s: %s", name, err))
		}

		if string(fc) != contents {
			panic(fmt.Errorf("contents of %s doesn't match '%s': %s", name, contents, string(fc)))
		}
	}

}

func startServer(sfs *fs.FS) io.ReadWriteCloser {
	p1r, p1w := io.Pipe()
	p2r, p2w := io.Pipe()
	pipe := &twoPipe{p2r, p1w}
	go go9p.ServeReadWriter(p1r, p2w, sfs.Server())
	return pipe
}

func newFS() (*fs.FS, *fs.StaticDir) {
	return fs.NewFS("glenda", "glenda", 0755)
}

func newStaticDir(f *fs.FS, name string) *fs.StaticDir {
	return fs.NewStaticDir(f.NewStat(name, "glenda", "glenda", 0555))
}

func newStaticFile(f *fs.FS, name string, content string) fs.File {
	return fs.NewStaticFile(f.NewStat(name, "glenda", "glenda", 0444), []byte(content))
}

func TestSingleRootMount(t *testing.T) {
	rootfs, rootfsdir := newFS()
	bindir := newStaticDir(rootfs, "bin")
	bindir.AddChild(newStaticFile(rootfs, "ls", "Binary data\n"))
	rootfsdir.AddChild(bindir)
	rootpipe := startServer(rootfs)
	defer rootpipe.Close()

	ufs := NewUnionFS()
	rootc := mustNewClient(rootpipe)
	mustMount(ufs, rootc, "/", REPLACE, false)

	bin := findDir(t, ufs.Root, "bin")
	assertFile(bin, "ls", "Binary data\n")

	err := UnmountClient(ufs, rootc, "/")
	if err != nil {
		panic(err)
	}

	if len(ufs.Root.Children()) != 0 {
		t.Fatalf("root has not be unmounted")
	}
}

func TestReplace(t *testing.T) {
	// This is the root ('/') with directories /bin (with ls) and /usr.
	rootfs, rootfsdir := newFS()
	bindir := newStaticDir(rootfs, "bin")
	bindir.AddChild(newStaticFile(rootfs, "ls", "Binary data\n"))
	rootfsdir.AddChild(bindir)
	rootfsdir.AddChild(newStaticDir(rootfs, "usr")) // Mount-point for /usr fs
	rootpipe := startServer(rootfs)
	defer rootpipe.Close()

	// This is a /usr filesystem
	usrfs, usrfsdir := newFS()
	usrbindir := newStaticDir(usrfs, "bin")
	usrbindir.AddChild(newStaticFile(usrfs, "cat", "More binary data\n"))
	usrfsdir.AddChild(usrbindir)
	usrpipe := startServer(usrfs)
	defer usrpipe.Close()

	ufs := NewUnionFS()

	rootc := mustNewClient(rootpipe)
	mustMount(ufs, rootc, "/", AFTER, false)
	defer UnmountClient(ufs, rootc, "/")

	usrc := mustNewClient(usrpipe)
	mustMount(ufs, usrc, "/usr", AFTER, false)
	defer UnmountClient(ufs, usrc, "/usr")

	// Replace /bin with /usr/bin
	mustBind(ufs, "/usr/bin", "/bin", REPLACE, false)
	bin := findDir(t, ufs.Root, "/bin")
	if len(bin.Children()) != 1 {
		t.Fatalf("/bin hasn't been replaced with /usr/bin")
	}
	assertFile(bin, "cat", "More binary data\n")

	// Shadow out the root with /bin, which still remembers its content from its mount table
	mustBind(ufs, "/bin", "/", REPLACE, false)
	if len(ufs.Root.Children()) != 1 {
		t.Fatalf("/ hasn't been replaced with shadowed content")
	}
	findFile(t, ufs.Root, "cat")

	// Undo the shadowing of the root
	err := UnmountBind(ufs, "/bin", "/")
	if err != nil {
		panic(err)
	}
	if len(ufs.Root.Children()) != 2 {
		t.Fatalf("/ hasn't been restored to its pre-shadowed content")
	}

	// Undo the replacement of /bin
	err = UnmountBind(ufs, "/usr/bin", "/bin")
	if err != nil {
		panic(err)
	}
	bin = findDir(t, ufs.Root, "/bin")
	findFile(t, bin, "ls")
}

func TestCreate(t *testing.T) {
	// This is the root ('/') with directories /bin (with ls) and /usr.
	rootfs, rootfsdir := newFS()
	bindir := newStaticDir(rootfs, "bin")
	bindir.AddChild(newStaticFile(rootfs, "ls", "Binary data\n"))
	rootfsdir.AddChild(bindir)
	rootpipe := startServer(rootfs)
	defer rootpipe.Close()

	// This is an overlay filesystem that permits the user to create
	//  directories in their own storage.
	overlayfs, _ := newFS()
	overlayfs.CreateDir = fs.CreateStaticDir
	overlaypipe := startServer(overlayfs)
	defer overlaypipe.Close()

	// This is a second overlay filesystem
	topfs, _ := newFS()
	topfs.CreateDir = fs.CreateStaticDir
	toppipe := startServer(topfs)
	defer toppipe.Close()

	ufs := NewUnionFS()

	// Mount / as read-only
	rootc := mustNewClient(rootpipe)
	mustMount(ufs, rootc, "/", BEFORE, false)
	defer UnmountClient(ufs, rootc, "/")

	_, err := ufs.CreateDir(ufs, ufs.Root, "glenda", "tmp", 0755, 0x80)
	if err == nil {
		t.Fatalf("permission was granted to create a directory on a read-only filesystem")
	}

	// Mount / using the overlay that permits creation of directories
	overlayc := mustNewClient(overlaypipe)
	mustMount(ufs, overlayc, "/", AFTER, true)
	defer UnmountClient(ufs, overlayc, "/")

	tmp, err := ufs.CreateDir(ufs, ufs.Root, "glenda", "tmp", 0755, 0x80)
	if err != nil {
		panic(err)
	}

	findDir(t, ufs.Root, "/tmp")
	findDir(t, ufs.Root, "/bin")

	_, err = ufs.CreateDir(ufs, tmp, "glenda", "glenda", 0555, 0x80)
	if err != nil {
		panic(err)
	}

	findDir(t, ufs.Root, "/tmp/glenda")

	// Mount / using the top-most fs that permits creation of directories
	topc := mustNewClient(toppipe)
	mustMount(ufs, topc, "/", BEFORE, true)

	_, err = ufs.CreateDir(ufs, ufs.Root, "glenda", "usr", 0755, 0x80)
	if err != nil {
		panic(err)
	}

	findDir(t, ufs.Root, "/usr")

	// Make sure that the /usr directory disappeared along with the
	//  top-level filesystem where it was created.
	UnmountClient(ufs, topc, "/")
	if _, ok := ufs.Root.Children()["usr"]; ok {
		t.Fatalf("usr was created in the wrong client")
	}
}

func TestUnionMountBin(t *testing.T) {
	// This is the root ('/') with directories /bin (with ls) and /usr.
	rootfs, rootfsdir := newFS()
	bindir := newStaticDir(rootfs, "bin")
	bindir.AddChild(newStaticFile(rootfs, "ls", "Binary data\n"))
	rootfsdir.AddChild(bindir)
	rootfsdir.AddChild(newStaticDir(rootfs, "usr")) // Mount-point for /usr fs
	rootpipe := startServer(rootfs)
	defer rootpipe.Close()

	// This is a /usr filesystem
	usrfs, usrfsdir := newFS()
	usrbindir := newStaticDir(usrfs, "bin")
	usrbindir.AddChild(newStaticFile(usrfs, "cat", "More binary data\n"))
	usrfsdir.AddChild(usrbindir)
	usrpipe := startServer(usrfs)
	defer usrpipe.Close()

	// Create a union filesystem
	ufs := NewUnionFS()

	// Mount /
	rootc := mustNewClient(rootpipe)
	mustMount(ufs, rootc, "/", REPLACE, false)
	defer UnmountClient(ufs, rootc, "/")

	// Mount /usr
	usrc := mustNewClient(usrpipe)
	mustMount(ufs, usrc, "/usr", REPLACE, false)

	// Check that /usr/bin is there and has cat from the mount
	usrbin := findDir(t, ufs.Root, "/usr/bin")
	findFile(t, usrbin, "cat")

	// Bind /usr/bin to /bin
	mustBind(ufs, "/usr/bin", "/bin", AFTER, false)

	// Verify that the union of all bins can be found in /bin
	bin := findDir(t, ufs.Root, "/bin")
	if len(bin.Children()) != 2 {
		t.Fatalf("/bin doesn't have the union of /bin and /usr/bin: %s", bin)
	}
	assertFile(bin, "ls", "Binary data\n")
	assertFile(bin, "cat", "More binary data\n")

	// Un-bind /usr/bin from /bin
	err := UnmountBind(ufs, "/usr/bin", "/bin")
	if err != nil {
		panic(err)
	}

	bin = findDir(t, ufs.Root, "/bin")
	if len(bin.Children()) != 1 {
		t.Fatalf("/usr/bin hasn't been unbound from /bin: %s", bin)
	}
	assertFile(bin, "ls", "Binary data\n")

	// Un-mount /usr
	err = UnmountClient(ufs, usrc, "/usr")
	if err != nil {
		panic(err)
	}

	usr := findDir(t, ufs.Root, "/usr")
	if len(usr.Children()) != 0 {
		t.Fatalf("/usr hasn't been unmounted")
	}
}
