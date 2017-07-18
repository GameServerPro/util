package netutil

import (
	"bytes"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"util/buffer"
)

var errConnectionClosed = errors.New("connection closed")

type MockAddr struct{}

func (addr MockAddr) Network() string { return "mock" }
func (addr MockAddr) String() string  { return "" }

type MockConn struct {
	closed      int32
	readLocker  sync.Mutex
	readBuf     *bytes.Buffer
	writeLocker sync.Mutex
	writeBuf    *bytes.Buffer
}

func NewMockConn() *MockConn {
	return &MockConn{
		readBuf:  bytes.NewBufferString(""),
		writeBuf: bytes.NewBufferString(""),
	}
}

func (conn *MockConn) Close() error {
	atomic.StoreInt32(&conn.closed, 1)
	return nil
}

func (conn *MockConn) isClosed() bool { return atomic.LoadInt32(&conn.closed) != 0 }

func (conn *MockConn) LocalAddr() net.Addr                { return MockAddr{} }
func (conn *MockConn) RemoteAddr() net.Addr               { return MockAddr{} }
func (conn *MockConn) SetDeadline(t time.Time) error      { return nil }
func (conn *MockConn) SetReadDeadline(t time.Time) error  { return nil }
func (conn *MockConn) SetWriteDeadline(t time.Time) error { return nil }

func (conn *MockConn) Read(b []byte) (n int, err error) {
	if conn.isClosed() {
		return 0, errConnectionClosed
	}
	conn.readLocker.Lock()
	defer conn.readLocker.Unlock()
	return conn.readBuf.Read(b)
}

func (conn *MockConn) Write(b []byte) (n int, err error) {
	if conn.isClosed() {
		return 0, errConnectionClosed
	}
	conn.writeLocker.Lock()
	defer conn.writeLocker.Unlock()
	return conn.writeBuf.Write(b)
}

func TestWriteOnlySession(t *testing.T) {
	var (
		conn                 = NewMockConn()
		ws                   = NewWSession(conn, "1", 1024)
		onNewSessionInvoked  = false
		onQuitSessionInvoked = false
		quit                 = make(chan struct{})
		onNewSession         = func() { onNewSessionInvoked = true }
		onQuitSession        = func() { onQuitSessionInvoked = true; quit <- struct{}{} }
	)
	go ws.Run(onNewSession, onQuitSession)

	bp := buffer.NewBufferPool(2)
	buf, fromp := bp.Get()
	defer bp.Put(buf, fromp)

	buf.Write([]byte("hello"))
	buf.Add(1)
	ws.Send(buf)

	buf2, fromp2 := bp.Get()
	defer bp.Put(buf2, fromp2)
	buf2.Write([]byte("world"))
	buf2.Add(1)
	ws.Send(buf2)

	ws.Quit()
	<-quit

	if !onNewSessionInvoked {
		t.Errorf("onNewSessionInvoked is false")
	}
	if !onQuitSessionInvoked {
		t.Errorf("onQuitSessionInvoked is false")
	}
	if conn.writeBuf.String() != "helloworld" {
		t.Errorf("unexpected write result: %s", conn.writeBuf.String())
	}
}

type MockReadStream struct {
	conn          net.Conn
	buf           []byte
	data          *bytes.Buffer
	encrypter     CrypterFunc
	encrypterChan chan CrypterFunc
	decrypter     CrypterFunc
}

func NewMockReadStream(conn net.Conn) *MockReadStream {
	return &MockReadStream{conn: conn, buf: make([]byte, 1), data: bytes.NewBufferString(""), encrypterChan: make(chan CrypterFunc, 1)}
}

func (rstream *MockReadStream) Conn() net.Conn { return rstream.conn }
func (rstream *MockReadStream) Read() (int, error) {
	total := 0
	for {
		n, err := rstream.conn.Read(rstream.buf)
		total += n
		rstream.data.Write(rstream.buf[:n])
		if err != nil {
			break
		}
		if n < len(rstream.buf) {
			break
		}
	}
	return total, nil
}
func (rstream *MockReadStream) SetEncrypter(encrypter CrypterFunc) { rstream.encrypterChan <- encrypter }
func (rstream *MockReadStream) SetDecrypter(decrypter CrypterFunc) { rstream.decrypter = decrypter }
func (rstream *MockReadStream) SetTimeout(d time.Duration)         {}

func TestRWSession(t *testing.T) {
	var (
		conn                 = NewMockConn()
		data                 = "recv some messages"
		rstream              = NewMockReadStream(conn)
		s                    = NewRWSession(conn, rstream, "1", 1024)
		onNewSessionInvoked  = false
		onQuitSessionInvoked = false
		quit                 = make(chan struct{})
		onNewSession         = func() { onNewSessionInvoked = true }
		onQuitSession        = func() { onQuitSessionInvoked = true; quit <- struct{}{} }
	)
	conn.readBuf = bytes.NewBufferString(data)
	go s.Run(onNewSession, onQuitSession)

	bp := buffer.NewBufferPool(2)
	buf, fromp := bp.Get()
	defer bp.Put(buf, fromp)

	buf.Write([]byte("hello"))
	buf.Add(1)
	s.Send(buf)

	buf2, fromp2 := bp.Get()
	defer bp.Put(buf2, fromp2)
	buf2.Write([]byte("world"))
	buf2.Add(1)
	s.Send(buf2)

	time.Sleep(time.Second)
	s.Quit()
	<-quit

	if !onNewSessionInvoked {
		t.Errorf("onNewSessionInvoked is false")
	}
	if !onQuitSessionInvoked {
		t.Errorf("onQuitSessionInvoked is false")
	}
	if conn.writeBuf.String() != "helloworld" {
		t.Errorf("unexpected write result: %s", conn.writeBuf.String())
	}

	if rstream.data.String() != data {
		t.Errorf("unexpected data: `%v'", rstream.data.String())
	}
}
