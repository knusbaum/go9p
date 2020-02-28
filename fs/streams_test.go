package fs

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var _ File = &StreamFile{}

func TestStream(t *testing.T) {
	assert := assert.New(t)

	s := NewStream(100, false)
	n, err := s.Write([]byte("Hello"))
	assert.NoError(err)
	assert.Equal(5, n)

	n, err = s.Write([]byte("Goodbye"))
	assert.NoError(err)
	assert.Equal(7, n)
}

func TestStreamReader(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		assert := assert.New(t)

		s := NewStream(100, false)
		r := s.AddReader()
		n, err := s.Write([]byte("Hello"))
		assert.NoError(err)
		assert.Equal(5, n)

		n, err = s.Write([]byte("Goodbye"))
		assert.NoError(err)
		assert.Equal(7, n)

		readbs := make([]byte, 100)
		n, err = r.Read(readbs)
		assert.NoError(err)
		assert.Equal(12, n)
		assert.Equal("HelloGoodbye", string(readbs[:n]))
	})

	t.Run("Timeout", func(t *testing.T) {
		assert := assert.New(t)

		s := NewStream(100, false)
		r := s.AddReader()
		for i := 0; i < 1000; i++ {
			n, err := s.Write([]byte("a"))
			assert.NoError(err)
			assert.Equal(1, n)
		}

		// rdr caches 100 Write calls' worth of data.
		readbs := make([]byte, 1000)
		n, err := r.Read(readbs)
		assert.NoError(err)
		assert.Equal(100, n)
		assert.Equal("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", string(readbs[:n]))

		n, err = r.Read(readbs)
		assert.Error(err)
	})

	t.Run("SlowReader", func(t *testing.T) {
		assert := assert.New(t)
		// Slow readers shouldn't get killed
		s := NewStream(100, false)
		r := s.AddReader()
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			for i := 0; i < 1000; i++ {
				n, err := s.Write([]byte("a"))
				assert.NoError(err)
				assert.Equal(1, n)
			}
			s.Close()
			wg.Done()
		}()

		wg.Add(1)
		go func() {
			bs := make([]byte, 2000)
			curr := bs
			read := 0
			for {
				n, err := r.Read(curr)
				if err != nil {
					break
				}
				curr = curr[n:]
				read += n
				time.Sleep(20 * time.Millisecond)
			}
			assert.Equal(1000, read)
			wg.Done()
		}()
		wg.Wait()
	})

	t.Run("Blocking", func(t *testing.T) {
		assert := assert.New(t)
		// Slow readers shouldn't get killed
		s := NewStream(2, true)
		r := s.AddReader()
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			for i := 0; i < 20; i++ {
				n, err := s.Write([]byte("a"))
				assert.NoError(err)
				assert.Equal(1, n)
			}
			s.Close()
			wg.Done()
		}()

		wg.Add(1)
		go func() {
			bs := make([]byte, 2000)
			curr := bs
			read := 0
			i := 0
			for {
				n, err := r.Read(curr)
				if err != nil {
					break
				}
				curr = curr[n:]
				read += n
				time.Sleep(200 * time.Millisecond)
				i += 1
			}
			assert.Equal(20, read)
			wg.Done()
		}()
		wg.Wait()
	})
}
