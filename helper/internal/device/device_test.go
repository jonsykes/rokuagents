package device

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.bug.st/serial/enumerator"

	"github.com/tonylook/managents/helper/internal/protocol"
)

const deviceHello = `{"v":1,"t":"hello","device":"managents","fw":"0.1.0","board":"e32r40t","w":480,"h":320,"proto":1}` + "\n"

// fakePort replays scripted device output, one chunk per Read, and records writes.
type fakePort struct {
	mu       sync.Mutex
	chunks   []string
	written  bytes.Buffer
	writeErr error
	closed   bool

	openErr error // returned by fakeOpener instead of the port
	opens   int   // attempts to open the port
	short   bool  // Write takes only half of the data and reports no error
}

func (p *fakePort) Read(buf []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.chunks) == 0 {
		time.Sleep(time.Millisecond) // a read timeout with no data
		return 0, nil
	}
	n := copy(buf, p.chunks[0])
	p.chunks = p.chunks[1:]
	return n, nil
}

func (p *fakePort) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.writeErr != nil {
		return 0, p.writeErr
	}
	if p.short {
		data = data[:len(data)/2]
	}
	return p.written.Write(data)
}

func (p *fakePort) Close() error                       { p.closed = true; return nil }
func (p *fakePort) SetReadTimeout(time.Duration) error { return nil }

type fakeOpener map[string]*fakePort

func (o fakeOpener) Open(name string) (Port, error) {
	port, ok := o[name]
	if !ok {
		return nil, errors.New("no such port")
	}
	port.opens++
	if port.openErr != nil {
		return nil, port.openErr
	}
	return port, nil
}

type fakeEnumerator []string

func (e fakeEnumerator) Candidates() ([]string, error) { return e, nil }

type failingEnumerator struct{ err error }

func (e *failingEnumerator) Candidates() ([]string, error) { return nil, e.err }

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func bufferLogger(buf *bytes.Buffer) *slog.Logger { return slog.New(slog.NewTextHandler(buf, nil)) }

// newManager returns a Manager over one fake port, listed under name, on which
// a silent probe gives up quickly.
func newManager(name string, port *fakePort, logger *slog.Logger) *Manager {
	return &Manager{
		Enumerator:       fakeEnumerator{name},
		Opener:           fakeOpener{name: port},
		Logger:           logger,
		HandshakeTimeout: 20 * time.Millisecond,
	}
}

func TestCandidatesFrom(t *testing.T) {
	ports := []*enumerator.PortDetails{
		{Name: "/dev/cu.usbserial-20", IsUSB: true, VID: "1a86"},
		{Name: "/dev/tty.usbserial-20", IsUSB: true, VID: "1a86"},
		{Name: "/dev/cu.printer", IsUSB: true, VID: "2C99"},
		{Name: "/dev/cu.usbserial-10", IsUSB: true, VID: "10C4"},
		{Name: "/dev/cu.Bluetooth-Incoming-Port"},
		{Name: "/dev/ttyUSB0", IsUSB: true, VID: "303A"},
	}
	tests := []struct {
		name           string
		goos           string
		includeUnknown bool
		want           []string
	}{
		{"known bridges only, sorted", "darwin", false, []string{"/dev/cu.usbserial-10", "/dev/cu.usbserial-20"}},
		{"unknown bridges last", "darwin", true, []string{"/dev/cu.usbserial-10", "/dev/cu.usbserial-20", "/dev/cu.printer"}},
		{"dial-in nodes only skipped on macOS", "linux", false,
			[]string{"/dev/cu.usbserial-10", "/dev/cu.usbserial-20", "/dev/tty.usbserial-20", "/dev/ttyUSB0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := candidatesFrom(ports, tt.goos, tt.includeUnknown); !slices.Equal(got, tt.want) {
				t.Errorf("candidates = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHandshakeSkipsBootNoise(t *testing.T) {
	port := &fakePort{chunks: []string{"ets Jun  8 2016 00:22:57\r\n", "rst:0x1 (POWER", "ON_RESET)\n", deviceHello[:30], deviceHello[30:]}}

	hello, err := handshake(port, time.Second)

	if err != nil {
		t.Fatal(err)
	}
	if hello.Board != "e32r40t" || hello.W != 480 {
		t.Errorf("hello = %+v", hello)
	}
	if !strings.HasPrefix(port.written.String(), `{"v":1,"t":"hello"}`) {
		t.Errorf("probe = %q", port.written.String())
	}
}

func TestHandshakeTimesOutOnSilentPort(t *testing.T) {
	_, err := handshake(&fakePort{chunks: []string{"some other device\n"}}, 50*time.Millisecond)
	if !errors.Is(err, ErrNotADisplay) {
		t.Errorf("err = %v, want ErrNotADisplay", err)
	}
}

func TestHandshakeReportsWriteErrors(t *testing.T) {
	unplugged := errors.New("device not configured")
	if _, err := handshake(&fakePort{writeErr: unplugged}, time.Second); !errors.Is(err, unplugged) {
		t.Errorf("err = %v, want %v", err, unplugged)
	}
}

func TestHandshakeRecoversFromOverlongGarbage(t *testing.T) {
	// Boot messages read at the wrong baud rate (5000 bytes, no newline) end
	// with the display's first hello; its answer to the next probe follows.
	garbage := strings.Repeat("\xf0", 500)
	var chunks []string
	for range 10 {
		chunks = append(chunks, garbage)
	}
	port := &fakePort{chunks: append(chunks, deviceHello, deviceHello)}

	hello, err := handshake(port, time.Second)
	if err != nil || hello.Board != "e32r40t" {
		t.Errorf("hello = %+v, err = %v", hello, err)
	}
}

func TestSendReportsShortWrites(t *testing.T) {
	display := &Display{Name: "/dev/display", port: &fakePort{short: true}}
	if err := display.Send([]byte("frame\n")); !errors.Is(err, io.ErrShortWrite) {
		t.Errorf("err = %v, want io.ErrShortWrite", err)
	}
}

func TestManagerConnectsBroadcastsAndDropsDisplays(t *testing.T) {
	display := &fakePort{chunks: []string{deviceHello}}
	other := &fakePort{}
	manager := &Manager{
		Enumerator: fakeEnumerator{"/dev/display", "/dev/other", "/dev/missing"},
		Opener:     fakeOpener{"/dev/display": display, "/dev/other": other},
		Logger:     quietLogger(),
		// Keep tests fast: a silent port gives up quickly.
		HandshakeTimeout: 50 * time.Millisecond,
	}
	start := time.Now()

	manager.Discover(start)
	if manager.Count() != 1 {
		t.Fatalf("connected = %d, want 1", manager.Count())
	}
	if !other.closed {
		t.Error("a port without a display must be closed")
	}

	display.written.Reset()
	manager.Broadcast([]byte("frame\n"))
	if display.written.String() != "frame\n" {
		t.Errorf("display got %q", display.written.String())
	}

	display.writeErr = errors.New("device not configured")
	manager.Broadcast([]byte("frame\n"))
	if manager.Count() != 0 || !display.closed {
		t.Error("an unplugged display must be dropped and closed")
	}
}

// probeStep is a discovery pass at an offset from the start: whether the port
// is listed then, and whether the pass must probe it.
type probeStep struct {
	at     time.Duration
	listed bool
	probed bool
}

func runProbeSteps(t *testing.T, manager *Manager, port *fakePort, name string, steps []probeStep) {
	t.Helper()
	start := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	for _, step := range steps {
		manager.Enumerator = fakeEnumerator{}
		if step.listed {
			manager.Enumerator = fakeEnumerator{name}
		}
		opens := port.opens
		manager.Discover(start.Add(step.at))
		if probed := port.opens > opens; probed != step.probed {
			t.Errorf("at +%v: probed = %v, want %v", step.at, probed, step.probed)
		}
	}
}

func TestManagerRetriesPortsThatAreNotDisplaysTwice(t *testing.T) {
	var log bytes.Buffer
	silent := &fakePort{}
	manager := newManager("/dev/silent", silent, bufferLogger(&log))

	runProbeSteps(t, manager, silent, "/dev/silent", []probeStep{
		{0, true, true},
		{5 * time.Second, true, false},
		{10 * time.Second, true, true},
		{69 * time.Second, true, false},
		{70 * time.Second, true, true},
		{10 * time.Minute, true, false},
		{11 * time.Minute, false, false}, // unplugged
		{11*time.Minute + 2*time.Second, true, true},
		{11*time.Minute + 4*time.Second, true, false},
	})

	if hints := strings.Count(log.String(), "managents flash"); hints != 2 {
		t.Errorf("flash hint logged %d times, want once per plug-in (2)\n%s", hints, log.String())
	}
}

func TestManagerKeepsRetryingTheFixedPort(t *testing.T) {
	silent := &fakePort{}
	manager := newManager("/dev/fixed", silent, quietLogger())
	manager.FixedPort = "/dev/fixed"

	runProbeSteps(t, manager, silent, "/dev/fixed", []probeStep{
		{0, true, true},
		{10 * time.Second, true, true},
		{70 * time.Second, true, true},
		{100 * time.Second, true, false},
		{130 * time.Second, true, true},
	})
}

func TestManagerRetriesABusyPortFromScratch(t *testing.T) {
	board := &fakePort{}
	manager := newManager("/dev/board", board, quietLogger())

	runProbeSteps(t, manager, board, "/dev/board", []probeStep{
		{0, true, true},
		{10 * time.Second, true, true},
	})
	board.openErr = ErrPortBusy // a flasher writes new firmware
	runProbeSteps(t, manager, board, "/dev/board", []probeStep{
		{70 * time.Second, true, true},
		{72 * time.Second, true, true}, // busy ports are tried on every pass
	})
	board.openErr = nil // the new firmware still boots: a fresh schedule, not the third and last probe
	runProbeSteps(t, manager, board, "/dev/board", []probeStep{
		{74 * time.Second, true, true},
	})
	board.chunks = []string{deviceHello}
	runProbeSteps(t, manager, board, "/dev/board", []probeStep{
		{84 * time.Second, true, true},
	})

	if manager.Count() != 1 {
		t.Errorf("connected = %d, want the reflashed display", manager.Count())
	}
}

func TestManagerLogsABusyPortOnce(t *testing.T) {
	var log bytes.Buffer
	board := &fakePort{openErr: ErrPortBusy}
	manager := newManager("/dev/board", board, bufferLogger(&log))
	start := time.Now()

	for i := range 3 {
		manager.Discover(start.Add(time.Duration(i) * 2 * time.Second))
	}
	if n := strings.Count(log.String(), "managents service stop"); n != 1 {
		t.Errorf("busy hint logged %d times, want 1\n%s", n, log.String())
	}
}

func TestManagerLogsAListingFailureOnce(t *testing.T) {
	var log bytes.Buffer
	enum := &failingEnumerator{err: errors.New("IOServiceGetMatchingServices failed")}
	manager := &Manager{Enumerator: enum, Logger: bufferLogger(&log)}
	start := time.Now()

	for i, failure := range []error{enum.err, enum.err, enum.err, nil, enum.err} {
		enum.err = failure
		manager.Discover(start.Add(time.Duration(i) * 2 * time.Second))
	}
	if n := strings.Count(log.String(), "listing serial ports failed"); n != 2 {
		t.Errorf("listing failure logged %d times, want 2 (once per episode)\n%s", n, log.String())
	}
}

func TestManagerCloseDisconnectsEveryDisplay(t *testing.T) {
	first, late := &fakePort{chunks: []string{deviceHello}}, &fakePort{chunks: []string{deviceHello}}
	manager := &Manager{
		Enumerator: fakeEnumerator{"/dev/first"},
		Opener:     fakeOpener{"/dev/first": first, "/dev/late": late},
		Logger:     quietLogger(),
	}
	manager.Discover(time.Now())

	manager.Close()
	if manager.Count() != 0 || !first.closed {
		t.Error("Close must disconnect the connected displays")
	}

	// A discovery pass that was running at shutdown finds one more display.
	manager.Enumerator = fakeEnumerator{"/dev/late"}
	manager.Discover(time.Now())
	if manager.Count() != 0 || !late.closed {
		t.Error("a display found after Close must be closed at once")
	}
}

func TestManagerChecksFirmwareCompatibility(t *testing.T) {
	const (
		outOfDate = "display firmware is out of date"
		newProto  = "display speaks a newer protocol"
		available = "firmware 0.3.0 available: run managents flash"
	)
	tests := []struct {
		name      string
		firmware  string
		proto     int
		flashable string // the firmware the helper carries
		want      string // the message logged, if any
	}{
		{"supported", protocol.MinFirmware, protocol.Version, "", ""},
		{"newer firmware", "9.0.0-2-gabc1234", protocol.Version, "", ""},
		{"older firmware", "0.0.9", protocol.Version, "", outOfDate},
		{"newer protocol", protocol.MinFirmware, protocol.Version + 1, "", newProto},
		{"development build", "f91cdc5", protocol.Version, "0.3.0", ""},
		{"untagged build", "0.0.0-dev", protocol.Version, "0.3.0", ""},
		{"update available", "0.2.0", protocol.Version, "0.3.0", available},
		{"up to date", "0.3.0", protocol.Version, "0.3.0", ""},
		{"newer than the helper's", "0.4.0", protocol.Version, "0.3.0", ""},
		{"helper without firmware", "0.2.0", protocol.Version, "", ""},
		{"helper with a development image", "0.2.0", protocol.Version, "f91cdc5", ""},
		{"out of date wins", "0.0.9", protocol.Version, "0.3.0", outOfDate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var log bytes.Buffer
			hello := fmt.Sprintf(`{"v":%d,"t":"hello","device":"managents","fw":%q,"board":"e32r40t","w":480,"h":320,"proto":%d}`+"\n",
				protocol.Version, tt.firmware, tt.proto)
			manager := newManager("/dev/display", &fakePort{chunks: []string{hello}}, bufferLogger(&log))
			manager.AvailableFirmware = tt.flashable

			manager.Discover(time.Now())

			if manager.Count() != 1 {
				t.Error("the display must be used anyway")
			}
			for _, message := range []string{outOfDate, newProto, available} {
				if logged := strings.Contains(log.String(), message); logged != (message == tt.want) {
					t.Errorf("%q logged = %v\n%s", message, logged, log.String())
				}
			}
		})
	}
}

func TestManagerMentionsAvailableFirmwareOncePerConnection(t *testing.T) {
	var log bytes.Buffer
	hello := `{"v":1,"t":"hello","device":"managents","fw":"0.2.0","board":"e32r40t","w":480,"h":320,"proto":1}` + "\n"
	manager := newManager("/dev/display", &fakePort{chunks: []string{hello}}, bufferLogger(&log))
	manager.AvailableFirmware = "0.3.0"

	manager.Discover(time.Now())
	manager.Discover(time.Now().Add(time.Hour))

	if n := strings.Count(log.String(), "available: run managents flash"); n != 1 {
		t.Errorf("hint logged %d times, want 1\n%s", n, log.String())
	}
}

func TestManagerFixedPort(t *testing.T) {
	display := &fakePort{chunks: []string{deviceHello}}
	manager := &Manager{
		Enumerator: fakeEnumerator{"/dev/ignored"},
		Opener:     fakeOpener{"/dev/fixed": display},
		Logger:     quietLogger(),
		FixedPort:  "/dev/fixed",
	}
	manager.Discover(time.Now())
	if manager.Count() != 1 {
		t.Errorf("connected = %d, want 1", manager.Count())
	}
}
