package flash

import (
	"errors"
	"fmt"
	"io"
	"time"

	"go.bug.st/serial"

	"github.com/tonylook/managents/helper/internal/device"
)

// Port is the serial port of the board, including the modem lines that the
// USB bridge wires to the chip's reset and boot pins.
type Port interface {
	// Read waits up to the read timeout for data. It returns 0, nil when the
	// timeout passes with nothing received.
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	SetReadTimeout(d time.Duration) error
	// SetBaudRate changes the speed of an open port.
	SetBaudRate(baud int) error
	// SetDTR and SetRTS drive the modem lines; true is asserted (low).
	SetDTR(asserted bool) error
	SetRTS(asserted bool) error
	// ResetInputBuffer drops what the port has received and nobody read.
	ResetInputBuffer() error
	Close() error
}

// OpenSerial opens the named serial port at the ROM bootloader's speed. It
// fails with device.ErrPortBusy when another program holds the port.
func OpenSerial(name string) (Port, error) {
	port, err := serial.Open(name, &serial.Mode{
		BaudRate:          RomBaud,
		DataBits:          8,
		Parity:            serial.NoParity,
		StopBits:          serial.OneStopBit,
		InitialStatusBits: &serial.ModemOutputBits{DTR: false, RTS: false},
	})
	var portErr *serial.PortError
	if errors.As(err, &portErr) && portErr.Code() == serial.PortBusy {
		return nil, fmt.Errorf("%s: %w", name, device.ErrPortBusy)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return serialPort{port}, nil
}

// serialPort adapts a go.bug.st port to Port.
type serialPort struct{ serial.Port }

// Write sends all of p. The library's Write on Unix writes once, and a full
// output buffer makes that a short write that would cut a packet in two.
func (p serialPort) Write(data []byte) (int, error) {
	sent := 0
	for sent < len(data) {
		n, err := p.Port.Write(data[sent:])
		sent += n
		if err != nil {
			return sent, err
		}
		if n == 0 {
			return sent, io.ErrShortWrite
		}
	}
	return sent, nil
}

func (p serialPort) SetBaudRate(baud int) error {
	return p.SetMode(&serial.Mode{BaudRate: baud, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit})
}
