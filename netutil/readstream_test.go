package netutil

import (
	"bytes"
	"testing"

	"github.com/GameServerPro/proto"
)

func bytesEuqal(b1, b2 []byte) bool { return string(b1) == string(b2) }

func TestEncodeDecodeLength(t *testing.T) {
	for i, tc := range []struct {
		length int
		result []byte
	}{
		{0, []byte{0, 0}},
		{1, []byte{1, 0}},
		{256, []byte{0, 1}},
		{65535, []byte{255, 255}},
	} {
		if got := proto.EncodeLength(tc.length, make([]byte, 2)); !bytesEuqal(got, tc.result) {
			t.Errorf("%dth: unexpected result=%v", i, got)
		}
		if got := proto.DecodeLength(tc.result); got != tc.length {
			t.Errorf("%dth: unexpected length=%d", i, got)
		}
	}
}

func TestReadStream(t *testing.T) {
	conn := NewMockConn()
	data := ""
	for _, s := range []string{"hello", "world"} {
		conn.readBuf.Write(proto.EncodeLength(len(s)+2, make([]byte, proto.DefaultByteNumForLength))) // len(s) + 2
		conn.readBuf.WriteString(s)
		data += s
	}
	result := bytes.NewBufferString("")
	rs := NewReadStream(conn, func(b []byte) { result.Write(b[2:]) }) // b[2:]
	_, err := rs.Read()
	_, err = rs.Read()
	if err != nil {
		t.Errorf("ReadStream read error: %v", err)
	}
	if result.String() != data {
		t.Errorf("unexpected result:`%s' data:'%s'", result.String(), data)
	}
}
