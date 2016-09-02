package net

import (
	"fmt"
	"net"
	"time"

	"github.com/GameServerPro/util/buffer"
)

const (
	DefaultWriteSize = 4096
)

type DecryptFunc func(dst, src []byte)
type EncryptFunc func(dst, src []byte)
type PacketHandler func(packet []byte)

type Stream interface {
	Conn() net.Conn
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

type StreamReadWriter interface {
	StreamReader
	StreamWriter
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
			//fmt.Printf("read loop err:%v\n", err)
			//rws.w.setClose()
		}

		if rws.w.getClose() {
			break
		}
	}

	fmt.Println("read loop stop")
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
				fmt.Printf("write loop err:%v\n", err)
				rws.w.setClose()
			} else {
				fmt.Printf("write loop write:%v\n", string(b.Bytes()))
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
			fmt.Printf("write loop remain err:%v\n", err)
			break
		}
	}

	fmt.Println("write loop stop")
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

	go rws.startReadLoop(startRead, endRead)
	go rws.startWriteLoop(startWrite, endWrite)

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

// new read write stream
func NewRWStream(conn net.Conn, writeSize int, handle PacketHandler) (rws *ReadWriteStream) {
	rws = new(ReadWriteStream)
	rws.r = newReadStream(conn)
	rws.r.SetPacketHandler(handle)
	rws.w = newWriteStream(conn, writeSize)

	return
}
