package device

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/tonylook/managents/helper/internal/protocol"
)

// retryDelays space out the probes of a port that did not answer as a
// display. A probe opens the port, which may reset the board behind it, and
// holds it for up to the handshake timeout. A display that was still booting
// answers a retry; any other device is left alone after the last one, until it
// is unplugged or seen busy (a flasher had it, so it may run new firmware).
var retryDelays = []time.Duration{10 * time.Second, time.Minute}

// Manager keeps the set of connected displays up to date and fans protocol
// lines out to all of them. Displays may come and go at any time.
type Manager struct {
	Enumerator Enumerator
	Opener     Opener
	Logger     *slog.Logger
	// FixedPort, when set, is the only port used (no discovery).
	FixedPort string
	// HandshakeTimeout defaults to DefaultHandshakeTimeout.
	HandshakeTimeout time.Duration
	// AvailableFirmware is the firmware version this helper can flash, if it
	// has one. Displays that report an older release are told about it.
	AvailableFirmware string

	mu       sync.Mutex // guards displays and closed, which are also used during discovery
	displays map[string]*Display
	closed   bool

	// Discovery state, used by Discover only.
	rejected    map[string]rejection // ports that failed the handshake
	busy        map[string]bool      // ports last found in use by another program
	listFailure string               // the last enumeration error, logged once
}

// rejection is the retry state of a port that failed the handshake.
type rejection struct {
	at       time.Time // the last failed probe
	failures int
}

// NewManager returns a Manager for the real serial ports of this computer.
func NewManager(logger *slog.Logger, fixedPort string) *Manager {
	return &Manager{Enumerator: USBEnumerator{}, Opener: SerialOpener{}, Logger: logger, FixedPort: fixedPort}
}

// Discover connects to displays that appeared since the last call. Probing a
// port can take seconds, so Discover must not run concurrently with itself;
// the other methods may be called meanwhile.
func (m *Manager) Discover(now time.Time) {
	candidates, err := m.candidates()
	m.reportListFailure(err)
	if err != nil {
		return
	}
	m.forgetUnlisted(candidates)
	for _, name := range candidates {
		if m.isConnected(name) || !m.probeDue(name, now) {
			continue
		}
		display, err := Connect(m.Opener, name, m.handshakeTimeout())
		m.setBusy(name, errors.Is(err, ErrPortBusy))
		switch {
		case err == nil:
			delete(m.rejected, name)
			if m.add(display) {
				m.Logger.Info("display connected", "port", name, "board", display.Info.Board,
					"firmware", display.Info.FW, "size", sizeOf(display))
				m.checkCompatibility(display)
			}
		case errors.Is(err, ErrNotADisplay):
			m.reject(name, now)
		default:
			m.Logger.Debug("port skipped", "port", name, "err", err) // busy or gone: retried at once
		}
	}
}

// Broadcast sends one line to every connected display, dropping the ones
// that fail (unplugged); they are picked up again by Discover.
func (m *Manager) Broadcast(line []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, display := range m.displays {
		if err := display.Send(line); err != nil {
			m.Logger.Info("display disconnected", "port", name, "err", err)
			display.Close()
			delete(m.displays, name)
		}
	}
}

// Count is the number of connected displays.
func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.displays)
}

// Close disconnects every display, including the ones that a discovery pass
// still running finds afterwards.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	for name, display := range m.displays {
		display.Close()
		delete(m.displays, name)
	}
}

func (m *Manager) handshakeTimeout() time.Duration {
	if m.HandshakeTimeout > 0 {
		return m.HandshakeTimeout
	}
	return DefaultHandshakeTimeout
}

func (m *Manager) candidates() ([]string, error) {
	if m.FixedPort != "" {
		return []string{m.FixedPort}, nil
	}
	return m.Enumerator.Candidates()
}

func (m *Manager) isConnected(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.displays[name]
	return ok
}

// add keeps a newly connected display, unless the Manager has been closed.
func (m *Manager) add(display *Display) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		display.Close()
		return false
	}
	if m.displays == nil {
		m.displays = make(map[string]*Display)
	}
	m.displays[display.Name] = display
	return true
}

// probeDue applies the retry schedule of a port that failed the handshake.
// The fixed port is the one the user named, so it keeps being retried at the
// last delay.
func (m *Manager) probeDue(name string, now time.Time) bool {
	r, ok := m.rejected[name]
	switch {
	case !ok:
		return true
	case r.failures > len(retryDelays) && name != m.FixedPort:
		return false
	default:
		return now.Sub(r.at) >= retryDelays[min(r.failures, len(retryDelays))-1]
	}
}

func (m *Manager) reject(name string, now time.Time) {
	if m.rejected == nil {
		m.rejected = make(map[string]rejection)
	}
	r := m.rejected[name]
	if r.failures == 0 {
		m.Logger.Info("serial port did not answer as a managents display; new board? run: managents flash", "port", name)
	}
	m.rejected[name] = rejection{at: now, failures: r.failures + 1}
}

// setBusy records whether another program holds the port and says so once.
// Seeing a port busy starts its retry schedule over: if a flasher held it, the
// device may be a display now.
func (m *Manager) setBusy(name string, busy bool) {
	if !busy {
		delete(m.busy, name)
		return
	}
	if !m.busy[name] {
		m.Logger.Info("serial port in use by another program; if it is the managents service: managents service stop",
			"port", name)
	}
	if m.busy == nil {
		m.busy = make(map[string]bool)
	}
	m.busy[name] = true
	delete(m.rejected, name)
}

// forgetUnlisted drops the state of ports that are gone: plugging a device in
// again starts its retry schedule over.
func (m *Manager) forgetUnlisted(candidates []string) {
	gone := func(name string) bool { return !slices.Contains(candidates, name) }
	maps.DeleteFunc(m.rejected, func(name string, _ rejection) bool { return gone(name) })
	maps.DeleteFunc(m.busy, func(name string, _ bool) bool { return gone(name) })
}

// reportListFailure logs an enumeration error when it first occurs instead of
// on every pass, which would grow a service's log without bound.
func (m *Manager) reportListFailure(err error) {
	message := ""
	if err != nil {
		message = err.Error()
	}
	if message != m.listFailure && message != "" {
		m.Logger.Warn("listing serial ports failed", "err", err)
	}
	m.listFailure = message
}

// checkCompatibility tells the user when the display or the helper needs an
// update, or when the helper can flash newer firmware. Either way the display is used as far as both understand each other.
func (m *Manager) checkCompatibility(display *Display) {
	info := display.Info
	switch {
	case protocol.OlderThan(info.FW, protocol.MinFirmware):
		m.Logger.Warn("display firmware is out of date; update it with: managents flash",
			"port", display.Name, "firmware", info.FW, "minimum", protocol.MinFirmware)
	case protocol.OlderThan(info.FW, m.AvailableFirmware):
		m.Logger.Info(fmt.Sprintf("firmware %s available: run managents flash", m.AvailableFirmware),
			"port", display.Name, "firmware", info.FW)
	}
	if info.Proto > protocol.Version {
		m.Logger.Info("display speaks a newer protocol than this helper; update the helper",
			"port", display.Name, "display_protocol", info.Proto, "helper_protocol", protocol.Version)
	}
}

func sizeOf(d *Display) string {
	return fmt.Sprintf("%dx%d", d.Info.W, d.Info.H)
}
