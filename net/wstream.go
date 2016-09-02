package net

import (
	"fmt"
	"net"
	"sync/atomic"

	"github.com/GameServerPro/util/buffer"
)

// WriteStream
type WriteStream struct {
	conn        net.Conn
	encrypt     EncryptFunc
	encryptChan chan EncryptFunc

	encryptBuf []byte

	writeClose chan struct{}
	writeChan  chan *buffer.Buffer

	closed int32
}

func (ws *WriteStream) Send(buf *buffer.Buffer) {
	if buf.Len() < 0 || ws.getClose() {
		return
	}

	buf.Add(1)
	fmt.Printf("WriteStream Send:%v\n", string(buf.Bytes()))
	ws.writeChan <- buf
}

func (ws *WriteStream) write(buf *buffer.Buffer) (n int, err error) {
	defer buf.Done()

	src := buf.Bytes()
	if encrypt := ws.encrypt; encrypt != nil && buf.Encrypt {
		if len(ws.encryptBuf) < len(src) {
			ws.encryptBuf = make([]byte, len(src))
		}
		encrypt(ws.encryptBuf, src)
		n, err = ws.conn.Write(ws.encryptBuf[:len(src)])
	} else {
		n, err = ws.conn.Write(src)
	}
	fmt.Printf("WriteStream write:%v\n", string(src))
	return
}

func (ws *WriteStream) SetEncrypt(enc EncryptFunc) {
	ws.encryptChan <- enc
}

func (ws *WriteStream) setClose() {
	atomic.StoreInt32(&ws.closed, 1)
}

func (ws *WriteStream) getClose() bool {
	return atomic.LoadInt32(&ws.closed) == 1
}

func newWriteStream(conn net.Conn, writeSize int) (w *WriteStream) {
	if writeSize <= 0 {
		writeSize = DefaultWriteSize
	}
	w = new(WriteStream)
	w.conn = conn
	w.encryptChan = make(chan EncryptFunc)
	w.writeChan = make(chan *buffer.Buffer, writeSize)
	w.encryptBuf = make([]byte, 0)
	return
}
