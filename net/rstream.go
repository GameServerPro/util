package net

import (
	"sync"
	"fmt"
	"time"
	"net"
)

// ReadStream
type ReadStream struct {
	conn net.Conn

	buf []byte

	timeout time.Duration

	handler PacketHandler

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
	rs.handler(rs.buf[:total])
	fmt.Printf("ReadStream Read:%v,%v\n", total, string(rs.buf[:total]))
	return total, nil
}

func (rs *ReadStream) SetDecrypt(dec DecryptFunc) {
	rs.decryptChan <- dec
}

func (rs *ReadStream) SetTimeout(t time.Duration) {
	rs.timeout = t
}

func (rs *ReadStream) SetPacketHandler(handle PacketHandler) {
	rs.handler = handle
}

func newReadStream(conn net.Conn) (r *ReadStream) {
	r = new(ReadStream)
	r.conn = conn
	r.buf = make([]byte, 4096)
	r.decryptChan = make(chan DecryptFunc)
	// todo
	return
}
