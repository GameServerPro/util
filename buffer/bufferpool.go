package buffer

import (
	"sync"
	"time"
)

const (
	MaxGCTime = time.Second
)

type BufferPool struct {
	pool       *sync.Pool
	collection chan *Buffer
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
		select {
		case bp.collection <- buf:
		default:
			// put buffer fail
			// this case will use temporary obj
			// and may cause memory fragmentation
		}
	}
}

func (bp *BufferPool) run() {
	for buf := range bp.collection {
		if buf.GC(MaxGCTime) {
			bp.pool.Put(buf)
		}
	}
}

func NewBufferPool(initSize, maxSize int) *BufferPool {
	if maxSize < initSize {
		maxSize = initSize
	}
	bp := &BufferPool{
		pool:       &sync.Pool{New: func() interface{} { return NewBuffer() }},
		collection: make(chan *Buffer, maxSize),
	}
	for i := 0; i < initSize; i++ {
		bp.pool.Put(bp.pool.New())
	}
	go bp.run()
	return bp
}