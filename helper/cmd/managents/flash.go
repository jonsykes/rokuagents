package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/tonylook/managents/helper/internal/device"
	"github.com/tonylook/managents/helper/internal/firmware"
	"github.com/tonylook/managents/helper/internal/flash"
	"github.com/tonylook/managents/helper/internal/protocol"
	"github.com/tonylook/managents/helper/internal/service"
)

// board is the only display board this helper has firmware for.
const board = "e32r40t"

// embeddedFirmware returns the firmware image built into this helper and its
// version. Both are empty if the helper was built without one.
func embeddedFirmware() (image []byte, version string) {
	image, version, _ = firmware.Embedded(board)
	return image, version
}

const (
	// bootWait is how long a flashed display gets to start and answer.
	bootWait = 10 * time.Second
	// portReleaseWait is how long a stopped service gets to let go of the port.
	portReleaseWait = 5 * time.Second
	// progressStep is the share of the image between two progress lines.
	progressStep = 10
)

// flasher is what flashCommand does its work with, so tests can replace the
// serial port, the service manager and the clock.
type flasher struct {
	ports   device.Enumerator
	service func() (service.Manager, error)
	// flashPort opens the named port, writes image at flash offset 0 and
	// closes the port again.
	flashPort func(ctx context.Context, name string, image []byte, opts flash.Options) (flash.Result, error)
	// confirm connects to the display behind the port and returns what it says.
	confirm func(name string) (protocol.Hello, error)
	sleep   func(time.Duration)
}

func flashCommand(ctx context.Context, inv *invocation) error {
	flags := inv.flags()
	port := portFlag(flags)
	imagePath := flags.String("image", "", "write this merged firmware image (flash offset 0x0) instead of the one built into managents")
	baud := flags.Int("baud", flash.DefaultBaud, "transfer speed; lower it (115200) if flashing fails")
	if err := inv.parse(flags); err != nil {
		return err
	}
	if *baud < flash.RomBaud {
		return inv.usageError(flags, "invalid value %d for -baud: at least %d", *baud, flash.RomBaud)
	}
	f := &flasher{
		ports: device.USBEnumerator{},
		service: func() (service.Manager, error) {
			env, err := service.CurrentEnv()
			if err != nil {
				return nil, err
			}
			return service.New(env, service.Exec{})
		},
		flashPort: flashSerialPort,
		confirm: func(name string) (protocol.Hello, error) {
			display, err := device.Connect(device.SerialOpener{}, name, device.DefaultHandshakeTimeout)
			if err != nil {
				return protocol.Hello{}, err
			}
			display.Close()
			return display.Info, nil
		},
		sleep: time.Sleep,
	}
	return f.run(ctx, inv.stdout, *port, *imagePath, *baud)
}

// flashSerialPort is flasher.flashPort for a real serial port.
func flashSerialPort(ctx context.Context, name string, image []byte, opts flash.Options) (flash.Result, error) {
	port, err := flash.OpenSerial(name)
	if err != nil {
		return flash.Result{}, err
	}
	res, err := flash.Write(ctx, port, image, 0, opts)
	return res, errors.Join(err, port.Close())
}

func (f *flasher) run(ctx context.Context, out io.Writer, portName, imagePath string, baud int) (err error) {
	image, version, err := loadImage(imagePath)
	if err != nil {
		return err
	}
	if portName == "" {
		if portName, err = f.pickPort(); err != nil {
			return err
		}
	}
	what := imagePath
	if imagePath == "" {
		what = "managents firmware " + version
	}
	fmt.Fprintf(out, "Flashing %s (%d bytes) to %s\n", strings.TrimSpace(what), len(image), portName)

	stopped, restart := f.stopService(out)
	if restart != nil {
		defer func() { err = errors.Join(err, restart()) }()
	}

	started := time.Now()
	var res flash.Result
	for attempt := 0; ; attempt++ {
		res, err = f.flashPort(ctx, portName, image, flash.Options{Baud: baud, Progress: progressPrinter(out)})
		// A service that was just stopped may need a moment to close the port.
		if !stopped || !errors.Is(err, device.ErrPortBusy) || attempt >= int(portReleaseWait/time.Second) {
			break
		}
		f.sleep(time.Second)
	}
	switch {
	case errors.Is(err, device.ErrPortBusy):
		return fmt.Errorf("%w: close any serial monitor or other flashing tool that uses %s, then try again", err, portName)
	case ctx.Err() != nil:
		return fmt.Errorf("interrupted: the display has no firmware until you run managents flash again (%w)", ctx.Err())
	case errors.Is(err, flash.ErrWrongChip):
		return err
	case err != nil:
		return fmt.Errorf("%w\nThe chip's bootloader cannot be damaged: run managents flash again, "+
			"with -baud 115200 if the transfer fails", err)
	}
	elapsed := time.Since(started).Round(100 * time.Millisecond)
	fmt.Fprintf(out, "Written and verified (MD5): %d bytes, %d compressed, in %s at %d baud\n",
		len(image), res.Compressed, elapsed, res.Baud)

	info, err := f.waitForDisplay(portName)
	if err != nil {
		return fmt.Errorf("the firmware was written and verified, but the display did not answer: %w\n"+
			"Unplug it and plug it in again; if it stays silent, run managents devices", err)
	}
	fmt.Fprintf(out, "managents display ready: board %s, firmware %s\n", info.Board, info.FW)
	return nil
}

// loadImage reads the image to flash: the file at path, or else the firmware
// built into this helper. The version is empty for a file.
func loadImage(path string) (image []byte, version string, err error) {
	if path != "" {
		image, err = os.ReadFile(path)
		if err == nil && len(image) == 0 {
			err = fmt.Errorf("%s is empty", path)
		}
		return image, "", err
	}
	if image, version = embeddedFirmware(); len(image) == 0 {
		return nil, "", errors.New("this helper was built without firmware; download a release or pass --image")
	}
	return image, version, nil
}

// pickPort returns the port of the only display candidate.
func (f *flasher) pickPort() (string, error) {
	ports, err := f.ports.Candidates()
	if err != nil {
		return "", err
	}
	switch len(ports) {
	case 0:
		return "", errors.New("no board found on a USB serial port: plug it in with a data cable, or name the port with --port")
	case 1:
		return ports[0], nil
	}
	return "", fmt.Errorf("several boards found, choose one with --port: %s", strings.Join(ports, ", "))
}

// stopService stops the background service if it runs, as it holds the port,
// and returns the function that starts it again. That function is nil if the
// service was not running.
func (f *flasher) stopService(out io.Writer) (stopped bool, restart func() error) {
	m, err := f.service()
	if err != nil {
		return false, nil // no service on this system
	}
	if state, err := m.Status(); err != nil || !state.Running {
		return false, nil
	}
	if err := m.Stop(); err != nil {
		// Carry on: the port is then probably busy, which is reported.
		fmt.Fprintf(out, "Could not stop the managents service (%v); trying anyway\n", err)
		return false, nil
	}
	fmt.Fprintln(out, "Stopped the managents service while flashing")
	return true, func() error {
		if err := m.Start(); err != nil {
			return fmt.Errorf("starting the managents service again: %w (run: managents service start)", err)
		}
		fmt.Fprintln(out, "Started the managents service again")
		return nil
	}
}

// waitForDisplay waits for the new firmware to boot and answer. Each try
// itself takes up to the handshake timeout, so the pause between tries is short.
func (f *flasher) waitForDisplay(name string) (info protocol.Hello, err error) {
	const pause = 500 * time.Millisecond
	deadline := time.Now().Add(bootWait)
	for range int(bootWait / pause) { // the count bounds tests, whose sleep takes no time
		if info, err = f.confirm(name); err == nil || !time.Now().Before(deadline) {
			break
		}
		f.sleep(pause)
	}
	return info, err
}

// progressPrinter prints a line per phase and per progressStep percent of the
// transfer: plain lines, so that logs and pipes stay readable.
func progressPrinter(out io.Writer) func(flash.Progress) {
	last := -1
	return func(p flash.Progress) {
		switch p.Phase {
		case flash.Connecting:
			fmt.Fprintln(out, "Resetting the board into its bootloader...")
		case flash.Erasing:
			fmt.Fprintln(out, "Erasing...")
		case flash.Writing:
			if p.Done == 0 || p.Total == 0 {
				fmt.Fprintln(out, "Writing...")
				return
			}
			if pct := p.Done * 100 / p.Total / progressStep * progressStep; pct > last {
				last = pct
				fmt.Fprintf(out, "  %d%%\n", pct)
			}
		case flash.Verifying:
			fmt.Fprintln(out, "Verifying...")
		case flash.Resetting:
			fmt.Fprintln(out, "Restarting the board...")
		}
	}
}
