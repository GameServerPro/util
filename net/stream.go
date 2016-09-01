package net

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GameServerPro/util/buffer"
)

const (
	DefaultWriteSize = 4096
)

type DecryptFunc func(dst, src []byte)
type EncryptFunc func(dst, src []byte)

type Stream interface {
	Read() (int, error)
	Write()
}

type StreamReader interface {
	Read() (int, error)
	SetDecrypt(DecryptFunc)
	SetTimeout(time.Duration)
}

type StreamWriter interface {
	Write() (int, error)
	SetEncrypt(EncryptFunc)
}

//type StreamReadWriter interface {
//	StreamReader
//	StreamWriter
//
//	StartReadLoop()
//	StartWriteLoop()
//}

// ReadStream
type ReadStream struct {
	conn net.Conn

	buf []byte

	timeout time.Duration

	decLocker   sync.Mutex
	decrypt     DecryptFunc
	decryptChan chan DecryptFunc
}

func (rs *ReadStream) Read() (n int, err error) {
	if rs.timeout > 0 {
		rs.conn.SetReadDeadline(time.Now().Add(rs.timeout))
	}
	// todo
	total := 0
	n, err = rs.conn.Read(rs.buf[:])
	total += n
	if err != nil {
		return total, err
	}
	return total, nil
}

func (rs *ReadStream) SetDecrypt(dec DecryptFunc) {
	rs.decryptChan <- dec
}

func (rs *ReadStream) SetTimeout(t time.Duration) {
	rs.timeout = t
}

func newReadStream(conn net.Conn) (r *ReadStream) {
	r = new(ReadStream)
	r.conn = conn
	r.buf = make([]byte, 4096)
	r.decryptChan = make(chan DecryptFunc)
	// todo
	return
}

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

// ReadWriteStream
type ReadWriteStream struct {
	r *ReadStream
	w *WriteStream
}

func (rws *ReadWriteStream) startReadLoop(startRead, endRead chan struct{}) {
	startRead <- struct{}{}
	for {
		_, err := rws.r.Read()
		if err != nil {
			rws.w.setClose()
		}

		if rws.w.getClose() {
			break
		}
	}

	endRead <- struct{}{}
}

func (rws *ReadWriteStream) startWriteLoop(startWrite, endWrite chan struct{}) {
	startWrite <- struct{}{}
	remain := 0
	for {
		if rws.w.getClose() {
			remain = len(rws.w.writeChan)
			break
		}

		select {
		case b := <-rws.w.writeChan:
			_, err := rws.w.write(b)
			if err != nil {
				rws.w.setClose()
			}
		case encrypt := <-rws.w.encryptChan:
			rws.w.encrypt = encrypt
		case <-time.After(time.Second):
		}
	}

	// process remain
	for i := 0; i < remain; i++ {
		b := <-rws.w.writeChan
		_, err := rws.w.write(b)
		if err != nil {
			break
		}
	}

	endWrite <- struct{}{}
}

func (rws *ReadWriteStream) Send(buf *buffer.Buffer) {
	rws.w.Send(buf)
}

func (rws *ReadWriteStream) Start(onStarted, onEnd func()) {
	startRead := make(chan struct{})
	startWrite := make(chan struct{})
	endRead := make(chan struct{})
	endWrite := make(chan struct{})

	rws.startReadLoop(startRead, endRead)
	rws.startWriteLoop(startWrite, endWrite)

	<-startRead
	<-startWrite

	if onStarted != nil {
		onStarted()
	}

	<-endRead
	<-endWrite

	if rws.w.conn != nil {
		rws.w.conn.Close()
	}

	if onEnd != nil {
		onEnd()
	}
}

func (rws *ReadWriteStream) Stop() {
	rws.w.setClose()
}

// new read stream
func NewRWStream(conn net.Conn, writeSize int) (rws *ReadWriteStream) {
	rws = new(ReadWriteStream)
	rws.r = newReadStream(conn)
	rws.w = newWriteStream(conn, writeSize)

	return
}
