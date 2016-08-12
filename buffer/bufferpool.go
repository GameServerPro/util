package buffer

import (
	"sync"
)

type BufferPool struct {
	pool       *sync.Pool
	collection chan *Buffer
}

func NewBufferPool(size int) *BufferPool {
	bp := &BufferPool{
		pool:       &sync.Pool{New: func() interface{} { return NewBuffer() }},
		collection: make(chan *Buffer, 40960),
	}
	for i := 0; i < size; i++ {
		bp.pool.Put(bp.pool.New())
	}
	go bp.run()
	return bp
}

func (bp *BufferPool) Get() (buf *Buffer, fromPool bool) {
	bufObj := bp.pool.Get()
	if bufObj == nil {
		buf = NewBuffer()
	} else {
		fromPool = true
		buf = bufObj.(*Buffer)
		buf.Reset()
	}
	return
}

func (bp *BufferPool) Put(buf *Buffer, fromPool bool) {
	if fromPool && buf != nil {
		bp.collection <- buf
	}
}

func (bp *BufferPool) run() {
	for buf := range bp.collection {
		buf.Wait()
		bp.pool.Put(buf)
	}
}
