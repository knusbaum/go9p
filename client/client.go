package client

import (
	"errors"
	"io"
	"log"
	//"path/filepath"
	"sync"
	"strings"

	"github.com/knusbaum/go9p/proto"
)

type Client struct {
	c       io.ReadWriteCloser
	rootFid uint32
	tags    []uint16
	lastTag uint16
	fids    []uint32
	lastFid uint32
	calls   map[uint16]chan proto.FCall
	closed bool
	sync.Mutex
}

type File struct {
	fid    uint32
	client *Client
	offset uint64
}

func (c *Client) worker() {
	defer c.c.Close()
	for {
		call, err := proto.ParseCall(c.c)
		if err != nil {
			log.Printf("Client Error: %s", err)
			c.Lock()
			c.closed = true
			c.Unlock()
			return
		}
		c.Lock()
		rchan := c.calls[call.GetTag()]
		c.Unlock()
		if rchan == nil {
			continue
		}
		rchan <- call
	}
}

func NewClient(c io.ReadWriteCloser, user, aname string) (*Client, error) {
	client := &Client{
		c:       c,
		rootFid: 0,
		tags:    nil,
		lastTag: 0,
		fids:    nil,
		lastFid: 0,
		calls:   make(map[uint16]chan proto.FCall),
	}
	attach := proto.TAttach{
		Header: proto.Header{proto.Tattach, 0},
		Fid:    0,
		Afid:   ^uint32(0), // NOFID for now.
		Uname:  user,
		Aname:  aname,
	}

	_, err := client.c.Write(attach.Compose())
	if err != nil {
		return nil, err
	}
	res, err := proto.ParseCall(client.c)
	if err != nil {
		return nil, err
	}
	if rerror, ok := res.(*proto.RError); ok {
		return nil, errors.New(rerror.Ename)
	}
	_, ok := res.(*proto.RAttach)
	if !ok {
		return nil, errors.New("Unexpected respons while attaching.")
	}
	go client.worker()
	return client, nil
}

func (c *Client) getResponse(call proto.FCall) (proto.FCall, error) {
	response := make(chan proto.FCall)
	c.Lock()
	//fmt.Println("Writing Call.")
	c.calls[call.GetTag()] = response
	_, err := c.c.Write(call.Compose())
	c.Unlock()
	if err != nil {
		return nil, err
	}
	//fmt.Println("Waiting on Response Channel.")
	r, ok := <-response
	if !ok {
		return nil, errors.New("RPC Error.")
	}
	return r, nil
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
	log.Println("Walk()")
	defer log.Println("Walk() Return")
	parts := removeBlank(strings.Split(path, "/"))
	newfid := c.takeFid()
	walk := proto.TWalk{
		Header: proto.Header{proto.Twalk, c.takeTag()},
		Fid:    c.rootFid,
		Newfid: newfid,
		Nwname: uint16(len(parts)),
		Wname:  parts,
	}
	//fmt.Println("Getting Walk Response.")
	res, err := c.getResponse(&walk)
	//fmt.Printf("TWalk Response: %#v, error: %#v\n", res, err)
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
	return newfid, nil
}

func (c *Client) clunkFid(fid uint32) {
	log.Println("Clunk()")
	defer log.Println("Clunk() Return")
	clunk := proto.TClunk{
		Header: proto.Header{proto.Tclunk, c.takeTag()},
		Fid:    fid,
	}
	//fmt.Println("Getting Clunk Response.")
	c.getResponse(&clunk) // TODO: do something with response and err?
	//fmt.Printf("TClunk Response: %#v, error %#v\n", response, err)
	c.returnFid(fid)
}

//func (c *Client) OpenFile(name string, flag int, perm os.FileMode) (*File, error) {
//
//}

func (c *Client) Open(path string) (*File, error) {
	log.Println("Open()")
	defer log.Println("Open() Return")
	newFid, err := c.walkFid(path)
	if err != nil {
		return nil, err
	}

	open := proto.TOpen{
		Header: proto.Header{proto.Topen, c.takeTag()},
		Fid: newFid,
		Mode: proto.Oread,
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
	_, ok := res.(*proto.ROpen)
	if !ok {
		c.clunkFid(newFid)
		return nil, errors.New("Unexpected response to TOpen.")
	}
	//c.clunkFid(newFid)
	return &File{
		fid: newFid,
		client: c,
		offset: 0,
	}, nil
}

func (f *File) Close() error {
	log.Println("Close()")
	defer log.Println("Close() Return")
	f.client.clunkFid(f.fid)
	return nil
}

func (f *File) Read(p []byte) (n int, err error) {
	log.Println("Read()")
	defer log.Println("Read() Return")
	read := proto.TRead{
		Header: proto.Header{proto.Tread, f.client.takeTag()},
		Fid: f.fid,
		Offset: f.offset,
		Count: uint32(len(p)),
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
		return 0, errors.New("Unexpected response to TOpen.")
	}
	
	n = copy(p, rresp.Data)
	if uint32(n) != rresp.Count {
		panic("Sent too much data.")
	}
	f.offset += uint64(n)
	return n, nil
}