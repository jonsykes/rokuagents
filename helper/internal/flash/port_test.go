package flash

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"go.bug.st/serial"
)

// shortPort accepts at most chunk bytes per Write, like a full output buffer.
type shortPort struct {
	serial.Port
	chunk int
	got   bytes.Buffer
	fail  error
}

func (p *shortPort) Write(data []byte) (int, error) {
	if p.fail != nil {
		return 0, p.fail
	}
	n := min(len(data), p.chunk)
	p.got.Write(data[:n])
	return n, nil
}

func TestSerialPortWritesEverything(t *testing.T) {
	inner := &shortPort{chunk: 300}
	data := bytes.Repeat([]byte{1, 2, 3, 4}, 260)
	n, err := serialPort{inner}.Write(data)
	if err != nil || n != len(data) || !bytes.Equal(inner.got.Bytes(), data) {
		t.Errorf("wrote %d of %d: %v", n, len(data), err)
	}
}

func TestSerialPortWriteErrors(t *testing.T) {
	if _, err := (serialPort{&shortPort{}}).Write([]byte{1}); !errors.Is(err, io.ErrShortWrite) {
		t.Errorf("a port that takes nothing: %v", err)
	}
	boom := errors.New("unplugged")
	if _, err := (serialPort{&shortPort{fail: boom}}).Write([]byte{1}); !errors.Is(err, boom) {
		t.Errorf("got %v", err)
	}
}
