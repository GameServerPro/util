package buffer

import (
	"bytes"
	"sync"
)

type Buffer struct {
	sync.WaitGroup
	*bytes.Buffer
	Encrypt bool
}

func NewBuffer() *Buffer {
	return &Buffer{
		Buffer:  bytes.NewBufferString(""),
		Encrypt: true,
	}
}

func (buf *Buffer) Reset() {
	buf.Buffer.Reset()
	buf.Encrypt = true
}
