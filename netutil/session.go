package netutil

import (
	"net"
	"time"
	"sync/atomic"

	"github.com/weikaishio/go-logger/logger"

	"util/buffer"
)

type CrypterFunc func(dst, src []byte)
type DecryptFunc func(dst,src []byte)

type Session interface {
	Id() string
	Send(*buffer.Buffer)
	Run(onNewSession, onQuitSession func())
	Quit()
	SetEncrypter(encrypter CrypterFunc)
	SetDecrypter(decrypter CrypterFunc)
}

// 用于只写的UDP连接,实现 net.Conn 接口
type UDPConn struct {
	remoteAddr *net.UDPAddr
	localConn  *net.UDPConn
}

func NewUDPConn(localConn *net.UDPConn, remoteAddr *net.UDPAddr) *UDPConn {
	return &UDPConn{
		localConn:  localConn,
		remoteAddr: remoteAddr,
	}
}

func (conn *UDPConn) Close() error                       { return nil }
func (conn *UDPConn) LocalAddr() net.Addr                { return conn.localConn.LocalAddr() }
func (conn *UDPConn) RemoteAddr() net.Addr               { return conn.remoteAddr }
func (conn *UDPConn) SetDeadline(t time.Time) error      { return nil }
func (conn *UDPConn) SetReadDeadline(t time.Time) error  { return nil }
func (conn *UDPConn) SetWriteDeadline(t time.Time) error { return nil }
func (conn *UDPConn) Read(b []byte) (int, error)         { panic("UDPConn: cannot Read") }
func (conn *UDPConn) Write(b []byte) (int, error) {
	return conn.localConn.WriteToUDP(b, conn.remoteAddr)
}

// 只写的Session
type WSession struct {
	conn   net.Conn
	id     string
	closed int32

	writeQuit chan struct{}
	writeChan chan *buffer.Buffer

	encryptBuf []byte

	encrypter     CrypterFunc
	encrypterChan chan CrypterFunc
}

func NewWSession(conn net.Conn, id string, conWriteSize int) *WSession {
	if conWriteSize <= 0 {
		conWriteSize = 4096
	}
	return &WSession{
		conn:          conn,
		id:            id,
		writeQuit:     make(chan struct{}),
		writeChan:     make(chan *buffer.Buffer, conWriteSize),
		encrypterChan: make(chan CrypterFunc, 1),
		encryptBuf:    make([]byte, 0),
	}
}

func (ws *WSession) Id() string                         { return ws.id }
func (ws *WSession) setClosed()                         { atomic.StoreInt32(&ws.closed, 1) }
func (ws *WSession) getClosed() bool                    { return atomic.LoadInt32(&ws.closed) == 1 }
func (ws *WSession) SetEncrypter(encrypter CrypterFunc) { ws.encrypterChan <- encrypter }

func (ws *WSession) Send(b *buffer.Buffer) {
	if b.Len() > 0 && !ws.getClosed() {
		ws.writeChan <- b
	}
}

func (ws *WSession) write(b *buffer.Buffer) (err error) {
	src := b.Bytes()
	if encrypter := ws.encrypter; encrypter != nil && b.Encrypt {
		if len(ws.encryptBuf) < len(src) {
			ws.encryptBuf = make([]byte, len(src))
		}
		encrypter(ws.encryptBuf, src)
		_, err = ws.conn.Write(ws.encryptBuf[:len(src)])
	} else {
		_, err = ws.conn.Write(src)
	}
	b.Done()
	return
}

func (ws *WSession) startWriteLoop(startWrite, endWrite chan<- struct{}) {
	startWrite <- struct{}{}
	remain := 0
	for {
		if ws.getClosed() {
			remain = len(ws.writeChan)
			break
		}
		select {
		case b := <-ws.writeChan:
			err := ws.write(b)
			if err != nil {
				ws.setClosed()
			}
		case encrypter := <-ws.encrypterChan:
			ws.encrypter = encrypter
		case <-time.After(time.Second):
		}
	}

	for i := 0; i < remain; i++ {
		b := <-ws.writeChan
		err := ws.write(b)
		if err != nil {
			break
		}
	}

	ws.conn.Close()
	logger.LogError("WSession startWriteLoop end")
	endWrite <- struct{}{}
}

func (ws *WSession) Run(onNewSession, onQuitSession func()) {
	startWrite := make(chan struct{})
	endWrite := make(chan struct{})

	go ws.startWriteLoop(startWrite, endWrite)
	<-startWrite

	if onNewSession != nil {
		onNewSession()
	}

	<-endWrite

	if ws.conn != nil {
		ws.conn.Close()
	}

	if onQuitSession != nil {
		onQuitSession()
	}
}

func (ws *WSession) Quit() {
	ws.setClosed()
}

// 可读可写的Session
type RWSession struct {
	*WSession
	rstream StreamReader
}

func NewRWSession(conn net.Conn, rstream StreamReader, id string, conWriteSize int) *RWSession {
	s := new(RWSession)
	s.WSession = NewWSession(conn, id, conWriteSize)
	s.rstream = rstream
	return s
}

func (s *RWSession) SetDecrypter(decrypter CrypterFunc) {
	s.rstream.SetDecrypter(decrypter)
}

func (s *RWSession) startReadLoop(startRead, endRead chan<- struct{}) {
	startRead <- struct{}{}
	for {
		_, err := s.rstream.Read()
		if err != nil {
			s.setClosed()
		}
		if s.getClosed() {
			break
		}
	}
	logger.LogError("RWSession startReadLoop end")
	endRead <- struct{}{}
}

func (s *RWSession) Run(onNewSession, onQuitSession func()) {
	startRead := make(chan struct{})
	startWrite := make(chan struct{})
	endRead := make(chan struct{})
	endWrite := make(chan struct{})

	go s.startReadLoop(startRead, endRead)
	go s.startWriteLoop(startWrite, endWrite)

	<-startRead
	<-startWrite

	if onNewSession != nil {
		onNewSession()
	}

	<-endRead
	<-endWrite

	if s.conn != nil {
		s.conn.Close()
	}

	if onQuitSession != nil {
		onQuitSession()
	}
}
