package proto

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func randHeader(t uint8) Header {
	return Header{t, uint16(rand.Int31n(65536))}
}

func randQid() Qid {
	return Qid{uint8(rand.Int31n(256)), rand.Uint32(), rand.Uint64()}
}

func TestMarshall(t *testing.T) {
	for _, tt := range []FCall{
		&TRVersion{randHeader(Tversion), rand.Uint32(), "version"},
		&TRVersion{randHeader(Rversion), rand.Uint32(), "version"},
		&TAuth{randHeader(Tauth), rand.Uint32(), "UNAME", "ANAME"},
		&RAuth{randHeader(Rauth), randQid()},
		&TAttach{randHeader(Tattach), rand.Uint32(), rand.Uint32(), "UNAME", "ANAME"},
		&RAttach{randHeader(Rattach), randQid()},
		&RError{randHeader(Rerror), "ERROR"},
		&TFlush{randHeader(Tflush), uint16(rand.Uint32())},
		&RFlush{randHeader(Rflush)},
		&TWalk{randHeader(Twalk), rand.Uint32(), rand.Uint32(), 2, []string{"wname1", "wname2"}},
		&RWalk{randHeader(Rwalk), 2, []Qid{randQid(), randQid()}},
		&TOpen{randHeader(Topen), rand.Uint32(), Mode(rand.Uint32())},
		&ROpen{randHeader(Ropen), randQid(), rand.Uint32()},
		&TCreate{randHeader(Tcreate), rand.Uint32(), "NAME", rand.Uint32(), uint8(rand.Uint32())},
		&RCreate{randHeader(Rcreate), randQid(), rand.Uint32()},
		&TRead{randHeader(Tread), rand.Uint32(), rand.Uint64(), rand.Uint32()},
		&RRead{randHeader(Rread), 10, make([]byte, 10)},
		&TWrite{randHeader(Twrite), rand.Uint32(), rand.Uint64(), 100, make([]byte, 100)},
		&RWrite{randHeader(Rwrite), 100},
		&TClunk{randHeader(Tclunk), rand.Uint32()},
		&RClunk{randHeader(Rclunk)},
		&TRemove{randHeader(Tremove), rand.Uint32()},
		&RRemove{randHeader(Rremove)},
		&TStat{randHeader(Tstat), rand.Uint32()},
		&RStat{randHeader(Rstat), Stat{
			uint16(rand.Uint32()),
			rand.Uint32(),
			randQid(),
			rand.Uint32(),
			rand.Uint32(),
			rand.Uint32(),
			rand.Uint64(),
			"NAME",
			"Uid",
			"Gid",
			"Muid",
		}},
		&TWstat{randHeader(Twstat), rand.Uint32(), Stat{
			uint16(rand.Uint32()),
			rand.Uint32(),
			randQid(),
			rand.Uint32(),
			rand.Uint32(),
			rand.Uint32(),
			rand.Uint64(),
			"NAME",
			"Uid",
			"Gid",
			"Muid",
		}},
		&RWstat{randHeader(Rwstat)},
	} {
		t.Run(reflect.TypeOf(tt).Elem().Name(), func(t *testing.T) {
			assert := assert.New(t)
			comp := tt.Compose()
			r := bytes.NewReader(comp)
			c, err := ParseCall(r)
			assert.NoError(err)
			assert.Equal(tt, c)
		})
	}
}

func TestBadMessage(t *testing.T) {
	t.Run("Random", func(t *testing.T) {
		assert := assert.New(t)
		bs := make([]byte, 1024)
		rand.Read(bs)
		fc, err := ParseCall(bytes.NewReader(bs))
		assert.Error(err)
		assert.Nil(fc)
	})
	t.Run("TooLong", func(t *testing.T) {
		assert := assert.New(t)
		bs := make([]byte, 1024)
		binary.LittleEndian.PutUint32(bs, 2000000000)
		fc, err := ParseCall(bytes.NewReader(bs))
		assert.Error(err)
		assert.Nil(fc)
	})
	t.Run("BadTag", func(t *testing.T) {
		assert := assert.New(t)
		bs := make([]byte, 1024)
		binary.LittleEndian.PutUint32(bs, 1024)
		bs[4] = 200
		fc, err := ParseCall(bytes.NewReader(bs))
		assert.Error(err)
		assert.Nil(fc)
	})
}
