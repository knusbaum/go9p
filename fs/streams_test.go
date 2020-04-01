package fs

import (
	"sync"
	"testing"
	"time"
	"os"
	"math/rand"

	"github.com/stretchr/testify/assert"
)

var testFile string = "/tmp/go9p.test.tmp"

var _ File = &StreamFile{}

var _ Stream = NewAsyncStream(100)
var _ Stream = NewBlockingStream(100, false)
var _ Stream = NewSkippingStream(50)
var _ Stream = &SavedStream{}

func TestStream(t *testing.T) {
	for name, sf := range map[string]func() Stream{
		"async":    func() Stream { return NewAsyncStream(50) },
		"blocking": func() Stream { return NewBlockingStream(50, false) },
		"skipping": func() Stream { return NewSkippingStream(50) },
		"saved": func() Stream { 
			os.Remove(testFile)
			s, err := NewSavedStream(testFile)
			assert.NoError(t, err)
			return s 
		},
	} {
		t.Run(name + "/ReadWrite", func(t *testing.T) {
			assert := assert.New(t)
			s := sf()

			n, err := s.Write([]byte("Hello"))
			assert.NoError(err)
			assert.Equal(5, n)

			n, err = s.Write([]byte("Goodbye"))
			assert.NoError(err)
			assert.Equal(7, n)
		})

		t.Run(name + "/normal", func(t *testing.T) {
			assert := assert.New(t)
			s := sf()
	
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

		t.Run(name + "/SlowReader", func(t *testing.T) {
			assert := assert.New(t)
			s := sf()

			// Slow readers shouldn't get killed
			r := s.AddReader()
			var wg sync.WaitGroup
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
					time.Sleep(10 * time.Millisecond)
				}
				assert.Equal(1000, read)
				wg.Done()
			}()
			wg.Wait()
		})
	}
}

func TestAsyncStream(t *testing.T) {
	t.Run("Timeout", func(t *testing.T) {
		assert := assert.New(t)

		s := NewAsyncStream(100)
		r := s.AddReader()
		for i := 0; i < 1000; i++ {
			n, err := s.Write([]byte("a"))
			assert.NoError(err)
			assert.Equal(1, n)
		}

		// reader caches 100 Write calls' worth of data.
		readbs := make([]byte, 1000)
		n, err := r.Read(readbs)
		assert.NoError(err)
		assert.Equal(100, n)
		assert.Equal("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", string(readbs[:n]))

		n, err = r.Read(readbs)
		assert.Error(err)
	})
}

func TestBlockingStream(t *testing.T) {
	t.Run("StoppedReader", func(t *testing.T) {
		assert := assert.New(t)
		s := NewBlockingStream(1, false)
		r := s.AddReader()
		var wg sync.WaitGroup
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

		time.Sleep(100 * time.Millisecond)
		s.RemoveReader(r)
		wg.Wait()

	})
}

func TestSkippingStream(t *testing.T) {
	t.Run("ReaderSkips", func(t *testing.T) {
		assert := assert.New(t)
		// Slow readers should get skipped
		s := NewSkippingStream(2)
		r := s.AddReader()
		s.Write([]byte("a"))
		s.Write([]byte("b"))
		s.Write([]byte("c"))
	
		bs := make([]byte, 2000)
		n, err := r.Read(bs)
		assert.NoError(err)
		assert.Equal(2, n)
		assert.Equal(bs[:n], []byte("ab"))
	})
}

func TestSavedStream(t *testing.T) {
	t.Run("ContentSaved", func(t *testing.T) {
		assert := assert.New(t)
		defer os.Remove(testFile)
		os.Remove(testFile)
		s, err := NewSavedStream(testFile)
		if !assert.NoError(err) {
			return
		}
		count := 10240
		bs := make([]byte, count)
		n, err := rand.Read(bs)
		if !assert.NoError(err) {
			return
		}
		if !assert.Equal(count, n) {
			return
		}

		n, err = s.Write(bs)
		if !assert.NoError(err) {
			return
		}
		if !assert.Equal(count, n) {
			return
		}
		s.Close()

		r := s.AddReader()
		result := make([]byte, count)
		n, err = r.Read(result)
		if !assert.NoError(err) {
			return
		}
		if !assert.Equal(count, n) {
			return
		}
		assert.Equal(bs, result)
	})
}