package client

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path"

	"strings"
	"sync"

	"github.com/Plan9-Archive/libauth"
	"github.com/emersion/go-sasl"
	"github.com/knusbaum/go9p"
	"github.com/knusbaum/go9p/proto"
)

func verboseLog(msg string, args ...interface{}) {
	if go9p.Verbose {
		log.Printf(msg, args...)
	}
}

const _NOFID = ^uint32(0)

type Client struct {
	c             io.ReadWriteCloser
	rootFid       uint32
	tags          []uint16
	lastTag       uint16
	fids          []uint32
	lastFid       uint32
	calls         map[uint16]chan proto.FCall
	closed        bool
	pathCacheLock sync.RWMutex
	pathCache     map[string]uint32
	msize         uint32
	sync.Mutex
}

type File struct {
	fid    uint32
	client *Client
	offset uint64
	iounit uint32
}

type Config struct {
	authFunc func(user string, s io.ReadWriter) (string, error)
}

type Option func(*Config)

func (c *Client) stop() {
	c.Lock()
	defer c.Unlock()
	c.closed = true
	c.c.Close()
}

func (c *Client) worker() {
	defer c.c.Close()
	for {
		call, err := proto.ParseCall(c.c)
		if err != nil {
			c.Lock()
			if c.closed {
				c.Unlock()
				return
			}
			c.closed = true
			c.Unlock()
			log.Printf("Client Error: %s", err)
			return
		}
		tag := call.GetTag()
		verboseLog("=in=> %v\n", call)
		c.Lock()
		rchan := c.calls[tag]
		c.Unlock()
		if rchan == nil {
			continue
		}
		rchan <- call
		c.returnTag(tag)
	}
}

func WithAuth(f func(user string, s io.ReadWriter) (string, error)) Option {
	return func(c *Config) {
		c.authFunc = f
	}
}

func Plan9Auth(user string, s io.ReadWriter) (string, error) {
	//log.Println("STARTING LIBAUTH PROXY")
	//defer log.Println("FINISHED LIBAUTH PROXY")
	ai, err := libauth.Proxy(s, "proto=p9any role=client user=%s", user)
	if err != nil {
		log.Printf("Authentication Error: %s", err)
		return "", err
	} else {
		log.Printf("AuthInfo: [Cuid: %s, Suid: %s, Cap: %s]", ai.Cuid, ai.Suid, ai.Cap)
		return ai.Cuid, nil
	}
}

func PlainAuth(password string) func(string, io.ReadWriter) (string, error) {
	return func(user string, s io.ReadWriter) (string, error) {
		client := sasl.NewPlainClient(user, user, password)
		_, ir, err := client.Start()
		if err != nil {
			return "", err
		}
		log.Printf("WRITE1\n")
		//s.Write([]byte(mech))
		var ba [4096]byte
		if ir != nil {
			// 			log.Printf("READ1\n")
			// 			_, err := s.Read(ba[:])
			// 			if err != nil {
			// 				return "", err
			// 			}
			// 			//bs := ba[:n]
			// 			log.Printf("WRITE2\n")
			log.Printf("WRITE1\n")
			s.Write(ir)
		}
		for {
			log.Printf("READ2\n")
			n, err := s.Read(ba[:])
			if err != nil {
				if err == io.EOF {
					return "", nil
				}
				return "", err
			}
			bs := ba[:n]
			resp, err := client.Next(bs)
			if err != nil {
				return "", err
			}
			log.Printf("WRITE3\n")
			s.Write(resp)
		}
	}
}

func NewClient(c io.ReadWriteCloser, user, aname string, opts ...Option) (*Client, error) {
	conf := Config{}
	for _, o := range opts {
		o(&conf)
	}
	client := &Client{
		c:         c,
		rootFid:   0,
		tags:      nil,
		lastTag:   1,
		fids:      nil,
		lastFid:   0,
		calls:     make(map[uint16]chan proto.FCall),
		pathCache: make(map[string]uint32),
	}
	var afid uint32 = _NOFID
	go client.worker()

	version := proto.TRVersion{
		Header:  proto.Header{proto.Tversion, 0},
		Msize:   65536,
		Version: "9P2000",
	}
	res, err := client.getResponse(&version)
	if err != nil {
		client.stop()
		return nil, err
	}
	if rerror, ok := res.(*proto.RError); ok {
		client.stop()
		return nil, errors.New(rerror.Ename)
	}
	ver, ok := res.(*proto.TRVersion)
	if !ok {
		client.stop()
		return nil, fmt.Errorf("Unexpected response while performing version: %v", res)
	}
	client.msize = ver.Msize

	if conf.authFunc != nil {
		afid = client.takeFid()
		// perform Authentication.
		auth := proto.TAuth{
			Header: proto.Header{proto.Tauth, 0},
			Afid:   afid,
			Uname:  user,
			Aname:  aname,
		}
		res, err := client.getResponse(&auth)
		if err != nil {
			client.stop()
			return nil, err
		}
		if rerror, ok := res.(*proto.RError); ok {
			client.stop()
			return nil, errors.New(rerror.Ename)
		}
		_, ok := res.(*proto.RAuth)
		if !ok {
			client.stop()
			return nil, fmt.Errorf("Unexpected response while performing auth: %v", res)
		}
		f := &File{
			fid:    afid,
			client: client,
			offset: 0,
			iounit: math.MaxUint32,
		}
		defer f.Close() // Needs to be closed *after* attach, or it becomes invalid
		conf.authFunc(user, f)
	}

	attach := proto.TAttach{
		Header: proto.Header{proto.Tattach, 0},
		Fid:    client.rootFid,
		Afid:   afid,
		Uname:  user,
		Aname:  aname,
	}

	res, err = client.getResponse(&attach)
	if err != nil {
		client.stop()
		return nil, err
	}
	if rerror, ok := res.(*proto.RError); ok {
		client.stop()
		return nil, fmt.Errorf("Failed to attach to filesystem: %v", rerror.Ename)
	}
	_, ok = res.(*proto.RAttach)
	if !ok {
		client.stop()
		return nil, fmt.Errorf("Unexpected response while attaching: %v", res)
	}

	return client, nil
}

func (c *Client) getResponse(call proto.FCall) (proto.FCall, error) {
	response := make(chan proto.FCall)
	c.Lock()
	c.calls[call.GetTag()] = response
	verboseLog("<=out= %v\n", call)
	_, err := c.c.Write(call.Compose())
	c.Unlock()
	if err != nil {
		return nil, err
	}
	r, ok := <-response
	if !ok {
		return nil, errors.New("RPC Error.")
	}
	return r, nil
}

func (c *Client) send(call proto.FCall) error {
	c.Lock()
	defer c.Unlock()
	verboseLog("<=out= %v\n", call)
	_, err := c.c.Write(call.Compose())
	return err
}

func (c *Client) takeTag() uint16 {
	c.Lock()
	defer c.Unlock()
	if len(c.tags) == 0 {
		c.lastTag++
		return c.lastTag
	}
	t := c.tags[len(c.tags)-1]
	c.tags = c.tags[:len(c.tags)-1]
	return t
}

func (c *Client) returnTag(tag uint16) {
	if tag == 0 {
		return
	}
	c.Lock()
	defer c.Unlock()
	c.tags = append(c.tags, tag)
	delete(c.calls, tag)
}

func (c *Client) takeFid() uint32 {
	c.Lock()
	defer c.Unlock()
	if len(c.fids) == 0 {
		c.lastFid++
		return c.lastFid
	}
	fid := c.fids[len(c.fids)-1]
	c.fids = c.fids[:len(c.fids)-1]
	return fid
}

func (c *Client) returnFid(fid uint32) {
	c.Lock()
	defer c.Unlock()
	c.fids = append(c.fids, fid)
}

func removeBlank(ss []string) []string {
	k := 0
	for _, s := range ss {
		if s != "" {
			ss[k] = s
			k++
		}
	}
	ss = ss[:k]
	return ss
}

// walkFid walks a new fid to the selected path from the root and returns it.
// fids should be returned to the client with returnFid once they're finished being used.
func (c *Client) walkFid(path string) (uint32, error) {
	//log.Printf("Walk(%s)", path)
	//defer log.Printf("Walk() Return ")
	parts := removeBlank(strings.Split(path, "/"))
	newfid := c.takeFid()
	walk := proto.TWalk{
		Header: proto.Header{proto.Twalk, c.takeTag()},
		Fid:    c.rootFid,
		Newfid: newfid,
		Nwname: uint16(len(parts)),
		Wname:  parts,
	}
	res, err := c.getResponse(&walk)
	if err != nil {
		c.clunkFid(newfid)
		return ^uint32(0), err
	}
	if rerror, ok := res.(*proto.RError); ok {
		c.clunkFid(newfid)
		return 0, errors.New(rerror.Ename)
	}
	_, ok := res.(*proto.RWalk)
	if !ok {
		c.clunkFid(newfid)
		return 0, errors.New("Unexpected response to TWalk.")
	}
	//log.Printf("Walk() Return (%d, nil)", newfid)
	return newfid, nil
}

func (c *Client) lookupFid(path string) (uint32, bool) {
	c.pathCacheLock.RLock()
	defer c.pathCacheLock.RUnlock()
	fid, ok := c.pathCache[path]
	return fid, ok
}

func (c *Client) cacheFid(path string) (uint32, error) {
	if fid, ok := c.lookupFid(path); ok {
		return fid, nil
	}
	fid, err := c.walkFid(path)
	if err != nil {
		return 0, err
	}
	c.pathCacheLock.Lock()
	defer c.pathCacheLock.Unlock()
	c.pathCache[path] = fid
	return fid, nil
}

func (c *Client) dropCachedFid(path string) {
	c.pathCacheLock.Lock()
	defer c.pathCacheLock.Unlock()
	delete(c.pathCache, path)
}

func (c *Client) clunkFid(fid uint32) {
	//log.Printf("Clunk(%d)", fid)
	//defer log.Println("Clunk() Return")
	if fid == 0 {
		panic("Clunked 0")
	}
	clunk := proto.TClunk{
		Header: proto.Header{proto.Tclunk, c.takeTag()},
		Fid:    fid,
	}
	//log.Println("Getting Clunk Response.")
	go func() {
		c.getResponse(&clunk) // TODO: do something with response and err?
		//c.send(&clunk)
		//log.Printf("TClunk Response: %#v, error %#v\n", response, err)
		c.returnFid(fid)
	}()
}

func readAll(max uint32, r io.Reader) ([]byte, error) {
	var buff bytes.Buffer
	buff.Grow(int(max))
	_, err := buff.ReadFrom(r)
	return buff.Bytes(), err
}

func (c *Client) Readdir(path string) ([]proto.Stat, error) {
	file, err := c.Open(path, proto.Oread)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	bs, err := readAll(c.msize, file)
	stats, err := proto.ParseStats(bs)
	if err != nil {
		//log.Printf("ERROR: %v\n", err)
		return nil, err
	}
	//log.Printf("STATS: %v\n", stats)
	return stats, nil
}

func (c *Client) Stat(path string) (*proto.Stat, error) {
	//log.Println("Stat()")
	//defer log.Println("Stat() Return")
	newFid, err := c.cacheFid(path)
	if err != nil {
		return nil, err
	}

	stat := proto.TStat{
		Header: proto.Header{proto.Tstat, c.takeTag()},
		Fid:    newFid,
	}
	res, err := c.getResponse(&stat)
	if err != nil {
		return nil, err
	}
	if rerror, ok := res.(*proto.RError); ok {
		return nil, errors.New(rerror.Ename)
	}
	rstat, ok := res.(*proto.RStat)
	if !ok {
		return nil, errors.New("Unexpected response to RStat.")
	}
	return &rstat.Stat, nil
}

func (c *Client) WStat(path string, stat *proto.Stat) error {
	//log.Println("WStat()")
	//defer log.Println("WStat() Return")
	newFid, err := c.cacheFid(path)
	if err != nil {
		return err
	}

	wstat := proto.TWstat{
		Header: proto.Header{proto.Twstat, c.takeTag()},
		Fid:    newFid,
		Stat:   *stat,
	}
	res, err := c.getResponse(&wstat)
	if err != nil {
		return err
	}
	if rerror, ok := res.(*proto.RError); ok {
		return errors.New(rerror.Ename)
	}
	_, ok := res.(*proto.RWstat)
	if !ok {
		return fmt.Errorf("Unexpected response to RWstat: %#v", res)
	}
	return nil
}

func (c *Client) Create(name string, perm os.FileMode) (*File, error) {
	//log.Printf("Create(%s)\n", name)
	//defer log.Println("Create() Return")
	newFid, err := c.walkFid(path.Dir(name))
	if err != nil {
		return nil, err
	}

	create := proto.TCreate{
		Header: proto.Header{proto.Tcreate, c.takeTag()},
		Fid:    newFid,
		Name:   path.Base(name),
		Perm:   uint32(perm),
		Mode:   uint8(proto.Ordwr),
	}
	res, err := c.getResponse(&create)
	if err != nil {
		c.clunkFid(newFid)
		return nil, err
	}
	if rerror, ok := res.(*proto.RError); ok {
		c.clunkFid(newFid)
		return nil, errors.New(rerror.Ename)
	}
	rc, ok := res.(*proto.RCreate)
	if !ok {
		c.clunkFid(newFid)
		return nil, errors.New("Unexpected response to TCreate.")
	}
	iounit := rc.Iounit
	if iounit == 0 {
		iounit = math.MaxUint32
	}
	return &File{
		fid:    newFid,
		client: c,
		offset: 0,
		iounit: iounit,
	}, nil
}

func (c *Client) Open(path string, mode proto.Mode) (*File, error) {
	//log.Println("Open()")
	//defer log.Println("Open() Return")
	newFid, err := c.walkFid(path)
	if err != nil {
		return nil, err
	}

	open := proto.TOpen{
		Header: proto.Header{proto.Topen, c.takeTag()},
		Fid:    newFid,
		Mode:   mode,
	}
	res, err := c.getResponse(&open)
	if err != nil {
		c.clunkFid(newFid)
		return nil, err
	}
	if rerror, ok := res.(*proto.RError); ok {
		c.clunkFid(newFid)
		return nil, errors.New(rerror.Ename)
	}
	ro, ok := res.(*proto.ROpen)
	if !ok {
		c.clunkFid(newFid)
		return nil, errors.New("Unexpected response to TOpen.")
	}
	//c.clunkFid(newFid)
	iounit := ro.Iounit
	if iounit == 0 {
		iounit = math.MaxUint32
	}
	return &File{
		fid:    newFid,
		client: c,
		offset: 0,
		iounit: iounit,
	}, nil
}

func (f *File) Close() error {
	//log.Println("Close()")
	//defer log.Println("Close() Return")
	f.client.clunkFid(f.fid)
	return nil
}

func (f *File) Read(p []byte) (n int, err error) {
	//log.Printf("Read(%d)", len(p))
	//defer log.Printf("Read() Return (%d, %v)", n, err)
	if len(p) > int(f.client.msize-11) {
		p = p[:f.client.msize-11]
	}
	if len(p) > int(f.iounit) {
		p = p[:f.iounit]
	}
	read := proto.TRead{
		Header: proto.Header{proto.Tread, f.client.takeTag()},
		Fid:    f.fid,
		Offset: f.offset,
		Count:  uint32(len(p)),
	}
	res, err := f.client.getResponse(&read)
	if err != nil {
		//c.clunkFid(newFid)
		return 0, err
	}
	if rerror, ok := res.(*proto.RError); ok {
		//c.clunkFid(newFid)
		return 0, errors.New(rerror.Ename)
	}
	rresp, ok := res.(*proto.RRead)
	if !ok {
		//c.clunkFid(newFid)
		return 0, errors.New("Unexpected response to TRead.")
	}
	//log.Printf("RRead <- %d", len(rresp.Data))

	n = copy(p, rresp.Data)
	if uint32(n) != rresp.Count {
		panic("Sent too much data.")
	}
	f.offset += uint64(n)
	if len(rresp.Data) == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func (f *File) ReadAt(b []byte, off int64) (n int, err error) {
	//log.Printf("ReadAt(%d (len: %d))\n", off, len(b))
	//defer func() { log.Printf("ReadAt -> %d, (err: %s)", n, err) }()
	if len(b) > int(f.client.msize-11) {
		b = b[:f.client.msize-11]
	}
	if len(b) > int(f.iounit) {
		b = b[:f.iounit]
	}
	read := proto.TRead{
		Header: proto.Header{proto.Tread, f.client.takeTag()},
		Fid:    f.fid,
		Offset: uint64(off),
		Count:  uint32(len(b)),
	}
	res, err := f.client.getResponse(&read)
	if err != nil {
		return 0, err
	}
	if rerror, ok := res.(*proto.RError); ok {
		return 0, errors.New(rerror.Ename)
	}
	rresp, ok := res.(*proto.RRead)
	if !ok {
		return 0, errors.New("Unexpected response to TRead.")
	}

	n = copy(b, rresp.Data)
	if uint32(n) != rresp.Count {
		panic("Sent too much data.")
	}
	f.offset += uint64(n)
	if len(rresp.Data) == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func (f *File) Write(p []byte) (n int, err error) {
	//log.Println("Write()")
	//defer log.Println("Write() Return")
	n, err = f.twrite(p, f.offset)
	f.offset += uint64(n)
	return n, err
}

func (f *File) WriteAt(b []byte, off int64) (n int, err error) {
	//log.Printf("WriteAt(b(%d), off: %d)", len(b), off)
	//defer log.Println("WriteAt() Return")
	return f.twrite(b, uint64(off))
}

func (f *File) twrite(p []byte, off uint64) (n int, err error) {
	wrote := 0
	for len(p) > 0 {
		//log.Printf("f.client.msize: %d, f.iounit: %d", f.client.msize, f.iounit)
		b := p
		if len(b) > int(f.client.msize-23) {
			b = b[:f.client.msize-23]
		}
		if len(b) > int(f.iounit) {
			b = b[:f.iounit]
		}
		write := proto.TWrite{
			Header: proto.Header{proto.Twrite, f.client.takeTag()},
			Fid:    f.fid,
			Offset: uint64(off) + uint64(wrote),
			Count:  uint32(len(b)),
			Data:   b,
		}
		res, err := f.client.getResponse(&write)
		if err != nil {
			return wrote, err
		}
		if rerror, ok := res.(*proto.RError); ok {
			return wrote, errors.New(rerror.Ename)
		}
		r, ok := res.(*proto.RWrite)
		if !ok {
			return wrote, errors.New("Unexpected response to TWrite.")
		}
		wrote += int(r.Count)
		p = p[r.Count:]
	}
	return wrote, nil
}

func (c *Client) Remove(path string) error {
	//log.Printf("Remove(%s)\n", path)
	//defer log.Println("Remove() Return")
	defer c.dropCachedFid(path)
	newFid, err := c.walkFid(path)
	if err != nil {
		return err
	}
	// Tremove automatically clunks the fid regardless of response.
	defer c.returnFid(newFid)

	remove := proto.TRemove{
		Header: proto.Header{proto.Tremove, c.takeTag()},
		Fid:    newFid,
	}
	res, err := c.getResponse(&remove)
	if err != nil {
		return err
	}
	if rerror, ok := res.(*proto.RError); ok {
		c.clunkFid(newFid)
		return errors.New(rerror.Ename)
	}
	_, ok := res.(*proto.RRemove)
	if !ok {
		return errors.New("Unexpected response to TRemove.")
	}
	return nil
}
