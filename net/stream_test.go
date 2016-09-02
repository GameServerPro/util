package net

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GameServerPro/util/buffer"
)

var (
	errConnectionClosed = errors.New("connection closed")
)

type MockAddr struct {
}

func (ma MockAddr) Network() string {
	return "mockAddr"
}
func (ma MockAddr) String() string {
	return "mockAddr"
}

type MockConn struct {
	closed int32
	rLock  sync.Mutex
	rbuf   *bytes.Buffer
	wLock  sync.Mutex
	wbuf   *bytes.Buffer
}

func (mc *MockConn) Read(b []byte) (int, error) {
	if mc.isClosed() {
		return 0, errConnectionClosed
	}
	mc.rLock.Lock()
	defer mc.rLock.Unlock()
	return mc.rbuf.Read(b)
}

func (mc *MockConn) Close() error {
	atomic.StoreInt32(&mc.closed, 1)
	return nil
}

func (mc *MockConn) Write(b []byte) (int, error) {
	if mc.isClosed() {
		return 0, errConnectionClosed
	}
	mc.wLock.Lock()
	defer mc.wLock.Unlock()
	return mc.rbuf.Write(b)
}

func (mc *MockConn) LocalAddr() net.Addr                { return MockAddr{} }
func (mc *MockConn) RemoteAddr() net.Addr               { return MockAddr{} }
func (mc *MockConn) SetDeadline(t time.Time) error      { return nil }
func (mc *MockConn) SetReadDeadline(t time.Time) error  { return nil }
func (mc *MockConn) SetWriteDeadline(t time.Time) error { return nil }
func (mc *MockConn) isClosed() bool                     { return atomic.LoadInt32(&mc.closed) == 1 }

func NewMockConn() *MockConn {
	return &MockConn{
		closed: 0,
		rbuf:   bytes.NewBufferString(""),
		wbuf:   bytes.NewBufferString(""),
	}
}

func Test_ReadWriteStream(t *testing.T) {
	var (
		conn      = NewMockConn()
		buf       = bytes.NewBufferString("")
		handle    = func(pack []byte) {
			fmt.Printf("testRWStream handle:%v\n",string(pack))
			buf.Write(pack)
		}
		started   = false
		end       = false
		stop      = make(chan struct{})
		onStarted = func() { started = true }
		onEnd     = func() { end = true; stop <- struct{}{} }
		bp        = buffer.NewBufferPool(2, 2)
	)

	conn.rbuf = bytes.NewBufferString("")
	rws := NewRWStream(conn, 1024, handle)

	fmt.Println("rwStream start...")
	go rws.Start(onStarted, onEnd)
	fmt.Println("rwStream started")

	buf1, f1 := bp.Get()
	buf2, f2 := bp.Get()
	defer bp.Put(buf1, f1)
	defer bp.Put(buf2, f2)

	buf1.Write([]byte("hello"))
	rws.Send(buf1)

	buf2.Write([]byte("world"))
	rws.Send(buf2)

	// wait one second and stop
	time.Sleep(time.Second * 5)
	fmt.Println("rwStream stop...")
	rws.Stop()
	fmt.Println("rwStream stoped")
	<-stop

	if !started {
		t.Error("on started must be callback")
	}

	if !end {
		t.Error("on end must be callback")
	}

	t.Log("this is read buf:", string(rws.r.buf))
	t.Log("this is write buf:", buf.String())
	if buf.String() != "helloworld" {
		t.Error("the write buf must be 'helloworld'")
	}

}
