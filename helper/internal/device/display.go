package device

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/tonylook/managents/helper/internal/protocol"
)

const (
	// DefaultHandshakeTimeout covers a display that reboots when the port
	// opens (about one second on ESP32 boards) and then answers.
	DefaultHandshakeTimeout = 3 * time.Second
	probeEvery              = time.Second
	readSlice               = 100 * time.Millisecond
)

// ErrNotADisplay means the port answered nothing that looks like a managents display.
var ErrNotADisplay = errors.New("no managents display on this port")

// Display is an open connection to one managents display.
type Display struct {
	Name string
	Info protocol.Hello
	port Port
}

// Connect opens the port and checks that a managents display is behind it.
// Its errors start with the port name; ErrPortBusy and ErrNotADisplay tell the
// common failures apart.
func Connect(opener Opener, name string, timeout time.Duration) (*Display, error) {
	port, err := opener.Open(name)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	hello, err := handshake(port, timeout)
	if err != nil {
		port.Close()
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return &Display{Name: name, Info: hello, port: port}, nil
}

// Send writes one encoded protocol line. Writing only part of it is an error
// (io.ErrShortWrite): the display would read the rest glued to the next line.
func (d *Display) Send(line []byte) error {
	n, err := d.port.Write(line)
	if err == nil && n < len(line) {
		return io.ErrShortWrite
	}
	return err
}

// Close releases the port.
func (d *Display) Close() error { return d.port.Close() }

// handshake probes with hello until the device identifies itself. Anything
// else on the line (boot messages, partial lines) is skipped.
func handshake(port Port, timeout time.Duration) (protocol.Hello, error) {
	if err := port.SetReadTimeout(readSlice); err != nil {
		return protocol.Hello{}, err
	}
	probe, err := protocol.Encode(protocol.HelloProbe())
	if err != nil {
		return protocol.Hello{}, err
	}

	deadline := time.Now().Add(timeout)
	var nextProbe time.Time
	var pending []byte
	buf := make([]byte, 512)
	for time.Now().Before(deadline) {
		if now := time.Now(); !now.Before(nextProbe) {
			if _, err := port.Write(probe); err != nil {
				return protocol.Hello{}, err
			}
			nextProbe = now.Add(probeEvery)
		}
		n, err := port.Read(buf)
		if err != nil {
			return protocol.Hello{}, err
		}
		pending = append(pending, buf[:n]...)
		for {
			i := bytes.IndexByte(pending, '\n')
			if i < 0 {
				break
			}
			if hello, ok := protocol.ParseDeviceHello(bytes.TrimSpace(pending[:i])); ok {
				return hello, nil
			}
			pending = pending[i+1:]
		}
		if len(pending) > protocol.MaxLineLength {
			pending = pending[:0]
		}
	}
	return protocol.Hello{}, ErrNotADisplay
}
