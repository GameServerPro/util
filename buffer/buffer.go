package buffer

import (
	"bytes"
	"sync/atomic"
	"time"
)

type Buffer struct {
	*bytes.Buffer
	Encrypt bool
	counter int32
}

func (buf *Buffer) Reset() {
	buf.Buffer.Reset()
	buf.Encrypt = true
	atomic.StoreInt32(&buf.counter, 0)
}

func (buf *Buffer) Add(n uint32) {
	atomic.AddInt32(&buf.counter, int32(n))
}

func (buf *Buffer) Done() {
	atomic.AddInt32(&buf.counter, -1)
}

func (buf *Buffer) GC(maxTime time.Duration) bool {
	start := time.Now()
	for {
		if atomic.LoadInt32(&buf.counter) == 0 {
			return true
		}
		if time.Now().Sub(start) >= maxTime {
			break
		}
		time.Sleep(time.Millisecond * 100)
	}
	return false
}

func NewBuffer() *Buffer {
	return &Buffer{
		Buffer:  bytes.NewBufferString(""),
		Encrypt: true,
		counter: 0,
	}
}
