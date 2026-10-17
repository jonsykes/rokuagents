package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tonylook/managents/helper/internal/device"
	"github.com/tonylook/managents/helper/internal/flash"
	"github.com/tonylook/managents/helper/internal/protocol"
	"github.com/tonylook/managents/helper/internal/service"
)

type fakePorts struct {
	names []string
	err   error
}

func (p fakePorts) Candidates() ([]string, error) { return p.names, p.err }

// fakeService records the calls the flasher makes.
type fakeService struct {
	running  bool
	stopErr  error
	startErr error
	calls    []string
}

func (s *fakeService) Install(string) error { return nil }
func (s *fakeService) Uninstall() error     { return nil }
func (s *fakeService) Start() error         { s.calls = append(s.calls, "start"); return s.startErr }
func (s *fakeService) Stop() error          { s.calls = append(s.calls, "stop"); return s.stopErr }
func (s *fakeService) Status() (service.State, error) {
	return service.State{Installed: true, Running: s.running}, nil
}

// flashRig is a flasher whose serial port and display are fakes.
type flashRig struct {
	*flasher
	svc       *fakeService
	flashErr  []error // result of each flashPort call; the last one repeats
	flashed   [][]byte
	flashedTo []string
	opts      flash.Options
	confirms  []error // result of each confirm call; the last one repeats
	slept     time.Duration
}

func newFlashRig(ports ...string) *flashRig {
	r := &flashRig{svc: &fakeService{}, flashErr: []error{nil}, confirms: []error{nil}}
	r.flasher = &flasher{
		ports:   fakePorts{names: ports},
		service: func() (service.Manager, error) { return r.svc, nil },
		flashPort: func(_ context.Context, name string, image []byte, opts flash.Options) (flash.Result, error) {
			r.flashed, r.flashedTo, r.opts = append(r.flashed, image), append(r.flashedTo, name), opts
			opts.Progress(flash.Progress{Phase: flash.Writing, Total: 100})
			opts.Progress(flash.Progress{Phase: flash.Writing, Done: 50, Total: 100})
			err := r.flashErr[0]
			if len(r.flashErr) > 1 {
				r.flashErr = r.flashErr[1:]
			}
			return flash.Result{Baud: opts.Baud, Compressed: 60}, err
		},
		confirm: func(string) (protocol.Hello, error) {
			err := r.confirms[0]
			if len(r.confirms) > 1 {
				r.confirms = r.confirms[1:]
			}
			return protocol.Hello{Board: "e32r40t", FW: "0.2.0"}, err
		},
		sleep: func(d time.Duration) { r.slept += d },
	}
	return r
}

func (r *flashRig) run(t *testing.T, port string) (string, error) {
	t.Helper()
	image := filepath.Join(t.TempDir(), "fw.bin")
	if err := os.WriteFile(image, []byte("firmware"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	err := r.flasher.run(context.Background(), &out, port, image, flash.DefaultBaud)
	return out.String(), err
}

func TestFlashWritesToTheOnlyBoard(t *testing.T) {
	r := newFlashRig("/dev/cu.usbserial-10")
	out, err := r.run(t, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.flashed) != 1 || string(r.flashed[0]) != "firmware" || r.flashedTo[0] != "/dev/cu.usbserial-10" {
		t.Errorf("flashed %q to %q", r.flashed, r.flashedTo)
	}
	if r.opts.Baud != flash.DefaultBaud {
		t.Errorf("baud %d", r.opts.Baud)
	}
	for _, want := range []string{"Flashing", "50%", "Written and verified", "managents display ready: board e32r40t, firmware 0.2.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if len(r.svc.calls) != 0 {
		t.Errorf("a stopped service was touched: %v", r.svc.calls)
	}
}

func TestFlashPortChoice(t *testing.T) {
	_, err := newFlashRig().run(t, "")
	if err == nil || !strings.Contains(err.Error(), "--port") {
		t.Errorf("no board: %v", err)
	}
	_, err = newFlashRig("/dev/a", "/dev/b").run(t, "")
	if err == nil || !strings.Contains(err.Error(), "/dev/a, /dev/b") {
		t.Errorf("two boards: %v", err)
	}
	r := newFlashRig("/dev/a", "/dev/b")
	if _, err := r.run(t, "/dev/b"); err != nil || r.flashedTo[0] != "/dev/b" {
		t.Errorf("explicit port: %v, %v", err, r.flashedTo)
	}
}

func TestFlashNeedsAnImage(t *testing.T) {
	// The tests are built without embedded firmware, as a plain checkout is.
	if image, _ := embeddedFirmware(); len(image) != 0 {
		t.Skip("firmware is embedded in this build")
	}
	r := newFlashRig("/dev/a")
	err := r.flasher.run(context.Background(), &strings.Builder{}, "", "", flash.DefaultBaud)
	if err == nil || !strings.Contains(err.Error(), "built without firmware") || !strings.Contains(err.Error(), "--image") {
		t.Errorf("got %v", err)
	}
	err = r.flasher.run(context.Background(), &strings.Builder{}, "", filepath.Join(t.TempDir(), "missing.bin"), flash.DefaultBaud)
	if err == nil || len(r.flashed) != 0 {
		t.Errorf("missing file: %v", err)
	}
}

func TestFlashStopsAndRestartsTheService(t *testing.T) {
	r := newFlashRig("/dev/a")
	r.svc.running = true
	out, err := r.run(t, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(r.svc.calls, ","); got != "stop,start" {
		t.Errorf("service calls %q", got)
	}
	if !strings.Contains(out, "Started the managents service again") {
		t.Error(out)
	}
}

func TestFlashRestartsTheServiceAfterAFailure(t *testing.T) {
	r := newFlashRig("/dev/a")
	r.svc.running = true
	r.flashErr = []error{flash.ErrVerify}
	_, err := r.run(t, "")
	if !errors.Is(err, flash.ErrVerify) {
		t.Fatalf("got %v", err)
	}
	if got := strings.Join(r.svc.calls, ","); got != "stop,start" {
		t.Errorf("service calls %q", got)
	}

	// Also when the display does not answer afterwards, and the restart's own
	// failure is reported next to it.
	r = newFlashRig("/dev/a")
	r.svc.running, r.svc.startErr = true, errors.New("launchctl failed")
	r.confirms = []error{device.ErrNotADisplay}
	_, err = r.run(t, "")
	if err == nil || !strings.Contains(err.Error(), "did not answer") || !strings.Contains(err.Error(), "launchctl failed") {
		t.Errorf("got %v", err)
	}
}

func TestFlashWaitsForAServiceToLetGoOfThePort(t *testing.T) {
	r := newFlashRig("/dev/a")
	r.svc.running = true
	r.flashErr = []error{device.ErrPortBusy, device.ErrPortBusy, nil}
	if _, err := r.run(t, ""); err != nil {
		t.Fatal(err)
	}
	if len(r.flashed) != 3 || r.slept < 2*time.Second {
		t.Errorf("%d tries, slept %v", len(r.flashed), r.slept)
	}
}

func TestFlashBusyPort(t *testing.T) {
	r := newFlashRig("/dev/a")
	r.flashErr = []error{device.ErrPortBusy}
	_, err := r.run(t, "")
	if !errors.Is(err, device.ErrPortBusy) || !strings.Contains(err.Error(), "serial monitor") {
		t.Errorf("got %v", err)
	}
	if len(r.flashed) != 1 {
		t.Errorf("%d tries for a port that another program holds", len(r.flashed))
	}
}

func TestFlashWaitsForTheDisplayToBoot(t *testing.T) {
	r := newFlashRig("/dev/a")
	r.confirms = []error{device.ErrNotADisplay, device.ErrNotADisplay, nil}
	out, err := r.run(t, "")
	if err != nil || !strings.Contains(out, "display ready") {
		t.Errorf("got %v\n%s", err, out)
	}
}

func TestFlashCommandRejectsALowBaud(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run(context.Background(), []string{"flash", "--baud", "9600"}, &stdout, &stderr)
	if code != exitUsage || !strings.Contains(stderr.String(), "-baud") {
		t.Errorf("exit %d: %s", code, stderr.String())
	}
}

func TestProgressPrinterSteps(t *testing.T) {
	var out strings.Builder
	p := progressPrinter(&out)
	p(flash.Progress{Phase: flash.Writing, Total: 1000})
	for done := 100; done <= 1000; done += 100 {
		p(flash.Progress{Phase: flash.Writing, Done: done, Total: 1000})
		p(flash.Progress{Phase: flash.Writing, Done: done, Total: 1000}) // a repeat prints nothing
	}
	if got := strings.Count(out.String(), "%"); got != 10 {
		t.Errorf("%d percent lines:\n%s", got, out.String())
	}
}
