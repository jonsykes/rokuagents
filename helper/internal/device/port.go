// Package device finds managents displays on USB serial ports and sends them
// protocol lines.
package device

import (
	"errors"
	"io"
	"runtime"
	"sort"
	"strings"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

// BaudRate is fixed by the protocol (115200 8N1).
const BaudRate = 115200

// Port is the part of a serial port the package needs.
type Port interface {
	io.ReadWriteCloser
	SetReadTimeout(time.Duration) error
}

// Opener opens serial ports by name. Open fails with ErrPortBusy when another
// program holds the port.
type Opener interface {
	Open(name string) (Port, error)
}

// ErrPortBusy means another program has the port open, for example a serial
// monitor, a flasher or another managents.
var ErrPortBusy = errors.New("serial port in use by another program")

// Enumerator lists serial ports that may have a display behind them.
type Enumerator interface {
	Candidates() ([]string, error)
}

// SerialOpener opens real serial ports.
type SerialOpener struct{}

// Open implements Opener. DTR and RTS stay low: on ESP32 boards they drive the
// auto-reset circuit, and asserting them would reboot the display.
func (SerialOpener) Open(name string) (Port, error) {
	port, err := serial.Open(name, &serial.Mode{
		BaudRate:          BaudRate,
		DataBits:          8,
		Parity:            serial.NoParity,
		StopBits:          serial.OneStopBit,
		InitialStatusBits: &serial.ModemOutputBits{DTR: false, RTS: false},
	})
	var portErr *serial.PortError
	if errors.As(err, &portErr) && portErr.Code() == serial.PortBusy {
		return nil, ErrPortBusy // the library opens ports exclusively
	}
	return port, err
}

// knownBridges are USB VIDs of the serial bridges found on ESP32 display
// boards. Discovery probes only ports behind them.
var knownBridges = map[string]string{
	"1A86": "WCH CH340/CH9102",
	"10C4": "Silicon Labs CP210x",
	"303A": "Espressif native USB",
	"0403": "FTDI",
}

// USBEnumerator lists the USB serial ports behind known ESP32 bridges.
type USBEnumerator struct {
	// IncludeUnknown also lists the ports behind other USB bridges, after the
	// known ones. Discovery leaves it off: a probe writes to the device and
	// may reset it, so other devices are only probed when the user asks.
	IncludeUnknown bool
}

// Candidates implements Enumerator.
func (e USBEnumerator) Candidates() ([]string, error) {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, err
	}
	return candidatesFrom(ports, runtime.GOOS, e.IncludeUnknown), nil
}

// candidatesFrom picks the USB call-out devices from ports: the ones behind
// known bridges sorted by name, then, with includeUnknown, the others.
func candidatesFrom(ports []*enumerator.PortDetails, goos string, includeUnknown bool) []string {
	var known, other []string
	for _, p := range ports {
		if !p.IsUSB || !isCallOutDevice(p.Name, goos) {
			continue
		}
		if _, ok := knownBridges[strings.ToUpper(p.VID)]; ok {
			known = append(known, p.Name)
		} else if includeUnknown {
			other = append(other, p.Name)
		}
	}
	sort.Strings(known)
	sort.Strings(other)
	return append(known, other...)
}

// isCallOutDevice skips macOS /dev/tty.* dial-in nodes, which block on open
// until carrier detect; every device also has a /dev/cu.* twin.
func isCallOutDevice(name, goos string) bool {
	return goos != "darwin" || strings.HasPrefix(name, "/dev/cu.")
}
