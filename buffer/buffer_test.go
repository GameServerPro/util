package buffer

import (
	"testing"
	"time"
)

func TestBuffer(t *testing.T) {
	b1 := NewBuffer()
	b2 := NewBuffer()

	b1.Add(1)
	b2.Add(1)

	go func() {
		time.Sleep(time.Millisecond * 500)
		b1.Done()

		time.Sleep(time.Millisecond * 500)
		b2.Done()
	}()

	if !b1.GC(time.Millisecond * 800) {
		t.Error("b1 should not gc")
	}

	if b2.GC(time.Millisecond * 800) {
		t.Log("b2 should gc")
	}

	if !b2.GC(time.Millisecond * 800) {
		t.Error("b2 should not gc")
	}
}
