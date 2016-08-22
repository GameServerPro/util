package netutil

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/GameServerPro/proto"
)

const (
	minRead = 512
)

type PacketHander func(packet []byte)

// 序列化后的数据流的读取
type StreamReader interface {
	Conn() net.Conn
	Read() (n int, err error)
	SetDecrypter(decrypter DecryptFunc)
	SetTimeout(d time.Duration)
}

type ReadStream struct {
	conn             net.Conn
	timeout          time.Duration
	bufForLength     []byte
	buf              []byte
	byteNumForLength int
	onRecvPacket     PacketHander
	decodeLengthFunc func([]byte) int

	decrypterLocker sync.RWMutex
	decrypter       DecryptFunc
}

// 通用的流读取
func NewReadStream(conn net.Conn, onRecvPacket PacketHander) *ReadStream {
	return &ReadStream{
		conn:             conn,
		buf:              make([]byte, 4096),
		byteNumForLength: proto.DefaultByteNumForLength,
		onRecvPacket:     onRecvPacket,
		decodeLengthFunc: proto.DecodeLength,
	}
}

func (r *ReadStream) Conn() net.Conn { return r.conn }

func (r *ReadStream) Read() (int, error) {
	total := 0
	// 读取 2 个字节以得到数据包的大小
	if r.timeout > 0 {
		r.conn.SetReadDeadline(time.Now().Add(r.timeout))
	}
	n, err := r.conn.Read(r.buf[:r.byteNumForLength])
	total += n
	if err != nil {
		return total, err
	}

	// 取得当前的 decrypter
	r.decrypterLocker.RLock()
	decrypter := r.decrypter
	r.decrypterLocker.RUnlock()

	// 取得包的大小
	if decrypter != nil {
		decrypter(r.buf[:r.byteNumForLength], r.buf[:r.byteNumForLength])
	}
	nextLength := r.decodeLengthFunc(r.buf[:r.byteNumForLength])
	readedSize := r.byteNumForLength
	if len(r.buf) < nextLength {
		r.buf = make([]byte, nextLength)
	}

	// 然后按包大小读取数据包
	for readedSize < nextLength {
		n, err := r.conn.Read(r.buf[readedSize:nextLength])
		total += n
		readedSize += n
		if err != nil {
			return total, err
		}
	}
	if decrypter != nil {
		decrypter(r.buf[r.byteNumForLength:nextLength], r.buf[r.byteNumForLength:nextLength])
	}
	r.onRecvPacket(r.buf[:nextLength])
	return total, nil
}

func (r *ReadStream) SetDecrypter(decrypter DecryptFunc) {
	r.decrypterLocker.Lock()
	defer r.decrypterLocker.Unlock()
	r.decrypter = decrypter
}

func (r *ReadStream) SetTimeout(d time.Duration) { r.timeout = d }

func (r *ReadStream) SetByteNumForLength(n int) {
	r.byteNumForLength = n
	if len(r.buf) < n {
		r.buf = make([]byte, n)
	}
}

func (r *ReadStream) SetDecodeLengthFunc(decodeLengthFunc func([]byte) int) {
	r.decodeLengthFunc = decodeLengthFunc
}

// UDP 包读取
type UDPReadStream struct {
	conn         *net.UDPConn
	onRecvPacket func([]byte)
	buf          []byte
	timeout      time.Duration
}

func NewUDPReadStream(conn *net.UDPConn, onRecvPacket PacketHander) *UDPReadStream {
	return &UDPReadStream{
		conn:         conn,
		onRecvPacket: onRecvPacket,
		buf:          make([]byte, 8192),
	}
}

func (r *UDPReadStream) Conn() net.Conn             { return r.conn }
func (r *UDPReadStream) Read(CrypterFunc) (int, error) {
	total := 0
	if r.timeout > 0 {
		r.conn.SetReadDeadline(time.Now().Add(r.timeout))
	}
	n, _, err := r.conn.ReadFromUDP(r.buf)
	total += n
	if err != nil && err != io.EOF {
		return total, err
	}
	if n > 0 {
		r.onRecvPacket(r.buf[:n])
	}
	return total, nil
}
func (r *UDPReadStream) SetDecrypter(CrypterFunc)   { panic("UDPReadStream unsupport decrypter") }
func (r *UDPReadStream) SetTimeout(d time.Duration) { r.timeout = d }

