package fs

import (
	"io"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var testFile string = "/tmp/go9p.test.tmp"

var _ File = &StreamFile{}

var _ BiDiStream = NewDroppingStream(100)
var _ BiDiStream = NewBlockingStream(100)
var _ BiDiStream = NewSkippingStream(50)
var _ Stream = &SavedStream{}

func TestStream(t *testing.T) {
	for name, sf := range map[string]func() Stream{
		"async":    func() Stream { return NewDroppingStream(50) },
		"blocking": func() Stream { return NewBlockingStream(50) },
		"skipping": func() Stream { return NewSkippingStream(50) },
		"saved": func() Stream {
			os.Remove(testFile)
			s, err := NewSavedStream(testFile)
			assert.NoError(t, err)
			return s
		},
	} {
		t.Run(name+"/ReadWrite", func(t *testing.T) {
			assert := assert.New(t)
			s := sf()

			n, err := s.Write([]byte("Hello"))
			assert.NoError(err)
			assert.Equal(5, n)

			n, err = s.Write([]byte("Goodbye"))
			assert.NoError(err)
			assert.Equal(7, n)
		})

		t.Run(name+"/normal", func(t *testing.T) {
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

		t.Run(name+"/SlowReader", func(t *testing.T) {
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
				defer wg.Done()
				bs := make([]byte, 2000)
				curr := bs
				read := 0
				for {
					n, err := r.Read(curr)
					if !assert.NoError(err) {
						return
					}
					curr = curr[n:]
					read += n
					if read == 1000 {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				assert.Equal(1000, read)
			}()
			wg.Wait()
		})
	}
}

func TestAsyncStream(t *testing.T) {
	t.Run("Timeout", func(t *testing.T) {
		assert := assert.New(t)

		s := NewDroppingStream(100)
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
		assert.NoError(err)
		assert.Equal(0, n)
	})
}

func TestBlockingStream(t *testing.T) {
	t.Run("StoppedReader", func(t *testing.T) {
		assert := assert.New(t)
		s := NewBlockingStream(1)
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

func TestBiDiStream(t *testing.T) {
	for name, sf := range map[string]func() BiDiStream{
		"async":    func() BiDiStream { return NewDroppingStream(50) },
		"blocking": func() BiDiStream { return NewBlockingStream(50) },
		"skipping": func() BiDiStream { return NewSkippingStream(50) },
	} {
		t.Run(name+"/BasicRead", func(t *testing.T) {
			assert := assert.New(t)
			s := sf()
			text := "The quick brown fox jumped over the lazy dog."
			s.AddReadWriter().Write([]byte(text))
			s.Close()

			var output []byte
			bs := make([]byte, 5)
			for {
				n, err := s.Read(bs)
				if err == io.EOF {
					break
				}
				if !assert.NoError(err) {
					return
				}
				output = append(output, bs[:n]...)
			}
			assert.Equal(text, string(output))
		})

		t.Run(name+"/NewReader", func(t *testing.T) {
			assert := assert.New(t)
			s := sf()

			read := make(chan []byte, 1)
			wait := make(chan struct{}, 0)
			go func() {
				bs := make([]byte, 1024)
				wait <- struct{}{}
				n, err := s.Read(bs)
				assert.NoError(err)
				read <- bs[:n]
			}()

			<-wait
			text := "The quick brown fox jumped over the lazy dog."
			s.AddReadWriter().Write([]byte(text))
			select {
			case t := <-read:
				assert.Equal(text, string(t))
			case <-time.After(2 * time.Second):
				assert.Fail("Timeout")
			}
		})

		t.Run(name+"/Chunked", func(t *testing.T) {
			assert := assert.New(t)
			s := sf()

			r1 := s.AddReadWriter()
			r2 := s.AddReadWriter()
			r3 := s.AddReadWriter()
			r4 := s.AddReadWriter()
			r1.Write([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
			r2.Write([]byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"))
			r3.Write([]byte("cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"))
			r4.Write([]byte("dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"))

			counts := make(map[byte]int)
			bs := make([]byte, 1)

			for i := 0; i < 4; i++ {
				var current byte
				// 60 of each character
				n, err := s.Read(bs)
				if !assert.Equal(1, n) || !assert.NoError(err) {
					return
				}
				current = bs[0]
				counts[current] += 1
				for j := 0; j < 59; j++ {
					n, err := s.Read(bs)
					if !assert.Equal(1, n) || !assert.NoError(err) || !assert.Equal(string(current), string(bs)) {
						return
					}
					counts[current]++
				}
			}
			assert.Equal(60, counts['a'])
			assert.Equal(60, counts['b'])
			assert.Equal(60, counts['c'])
			assert.Equal(60, counts['d'])
		})

		t.Run(name+"/MultiWrite", func(t *testing.T) {
			assert := assert.New(t)
			s := sf()

			r1 := s.AddReadWriter()
			r2 := s.AddReadWriter()
			r3 := s.AddReadWriter()
			r4 := s.AddReadWriter()
			r1.Write([]byte("a"))
			r2.Write([]byte("ab"))
			r3.Write([]byte("abc"))
			r4.Write([]byte("abcd"))

			// Writes may show up in any order, but they should come separately.
			check := func(bs []byte) {
				target := []byte("abcd")
				target = target[:len(bs)]
				assert.Equal(target, bs)
			}

			bs := make([]byte, 2000)
			n, err := s.Read(bs)
			assert.NoError(err)
			check(bs[:n])

			n, err = s.Read(bs)
			assert.NoError(err)
			check(bs[:n])

			n, err = s.Read(bs)
			assert.NoError(err)
			check(bs[:n])

			n, err = s.Read(bs)
			assert.NoError(err)
			check(bs[:n])
		})
	}
}
