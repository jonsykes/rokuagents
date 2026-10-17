package flash

import (
	"bytes"
	"context"
	"errors"
	"math/rand"
	"slices"
	"strings"
	"testing"
	"time"
)

// testImage is compressible (like code) and not (like data), with a length
// that needs padding.
func testImage(n int) []byte {
	rng := rand.New(rand.NewSource(1))
	image := make([]byte, n)
	for i := range image {
		if i/4096%2 == 0 {
			image[i] = byte(i / 7)
		} else {
			image[i] = byte(rng.Intn(256))
		}
	}
	return image
}

func noSleep(time.Duration) {}

func write(f *fakeROM, image []byte, offset uint32, opts Options) (Result, error) {
	opts.sleep = noSleep
	return Write(context.Background(), f, image, offset, opts)
}

func TestWriteFlashesTheImageExactly(t *testing.T) {
	for _, offset := range []uint32{0, 0x10000} {
		f := newFakeROM(t)
		image := testImage(100003) // not a multiple of four
		var progress []Progress
		res, err := write(f, image, offset, Options{Progress: func(p Progress) { progress = append(progress, p) }})
		if err != nil {
			t.Fatalf("offset 0x%x: %v", offset, err)
		}

		padded := append(slices.Clone(image), 0xff)
		for len(padded)%4 != 0 {
			padded = append(padded, 0xff)
		}
		if !bytes.Equal(f.flash[offset:offset+uint32(len(padded))], padded) {
			t.Errorf("offset 0x%x: the flash does not hold the image", offset)
		}
		for i, b := range f.flash[:offset] {
			if b != 0xff {
				t.Fatalf("flash byte 0x%x before the image changed", i)
			}
		}
		if f.flash[offset+uint32(len(padded))] != 0xff {
			t.Error("the flash after the image changed")
		}
		if want := uint32((len(padded) + 0x3ff) / 0x400 * 0x400); f.begin.size != want {
			t.Errorf("erase size %d, want %d (rounded to the block size)", f.begin.size, want)
		}
		if f.begin.blockSize != 0x400 || f.begin.offset != offset {
			t.Errorf("begin: %+v", f.begin)
		}
		if int(f.begin.blocks) != (res.Compressed+0x3ff)/0x400 {
			t.Errorf("%d blocks for %d compressed bytes", f.begin.blocks, res.Compressed)
		}
		if res.Compressed >= len(image) || res.Compressed != len(f.stream) {
			t.Errorf("compressed %d bytes, the chip got %d", res.Compressed, len(f.stream))
		}
		if res.Baud != DefaultBaud || f.baudChange != DefaultBaud || f.portBaud != DefaultBaud {
			t.Errorf("baud: result %d, chip told %d, port %d", res.Baud, f.baudChange, f.portBaud)
		}
		if !f.hardReset {
			t.Error("the chip was not reset into the new firmware")
		}
		if f.rts || f.dtr {
			t.Error("a modem line stays asserted after the flash")
		}
		checkProgress(t, progress, res.Compressed)
	}
}

func checkProgress(t *testing.T, progress []Progress, total int) {
	t.Helper()
	var phases []Phase
	done := 0
	for _, p := range progress {
		if len(phases) == 0 || phases[len(phases)-1] != p.Phase {
			phases = append(phases, p.Phase)
		}
		if p.Phase == Writing {
			if p.Total != total || p.Done < done {
				t.Fatalf("writing progress %+v after %d of %d", p, done, total)
			}
			done = p.Done
		}
	}
	if want := []Phase{Connecting, Erasing, Writing, Verifying, Resetting}; !slices.Equal(phases, want) {
		t.Errorf("phases %v, want %v", phases, want)
	}
	if done != total {
		t.Errorf("progress ended at %d of %d", done, total)
	}
}

func TestWriteChecksTheBootloaderCommandOrder(t *testing.T) {
	f := newFakeROM(t)
	if _, err := write(f, testImage(5000), 0, Options{Baud: RomBaud}); err != nil {
		t.Fatal(err)
	}
	if f.baudChange != 0 || f.portBaud != RomBaud {
		t.Errorf("the speed changed (to %d) although the ROM's speed was asked for", f.baudChange)
	}
	// Whatever the repeats, the first of each step comes in this order.
	var order []byte
	for _, op := range f.commands {
		if !slices.Contains(order, op) {
			order = append(order, op)
		}
	}
	want := []byte{opSync, opReadReg, opSpiAttach, opSpiSetParams, opDeflBegin, opDeflData, opSpiFlashMD5}
	if !bytes.Equal(order, want) {
		t.Errorf("commands %x, want %x", order, want)
	}
}

func TestWriteSurvivesNoiseAndSlowSync(t *testing.T) {
	f := newFakeROM(t)
	f.ignoreSyncs = 12 // the first reset's five syncs fail, so do those after the second
	f.out = []byte{0x00, 0xff, 'h', 'i', slipEnd, 0x01, 0x02, slipEnd, slipEnd, 0x55, slipEsc, slipEscEsc, slipEnd}
	if _, err := write(f, testImage(3000), 0, Options{}); err != nil {
		t.Fatal(err)
	}
	if f.resets < 3 {
		t.Errorf("%d resets, expected retries", f.resets)
	}
}

func TestWriteRetriesALostBlock(t *testing.T) {
	f := newFakeROM(t)
	f.failDataOnce[2] = 0x07 // checksum error
	image := testImage(60000)
	if _, err := write(f, image, 0, Options{}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(f.flash[:len(image)], image) {
		t.Error("flash differs after a retried block")
	}
}

func TestWriteStatusError(t *testing.T) {
	f := newFakeROM(t)
	f.failOn[opDeflBegin] = 0x08
	_, err := write(f, testImage(3000), 0, Options{})
	var status *StatusError
	if !errors.As(err, &status) || status.Code != 0x108 {
		t.Fatalf("got %v, want a StatusError 0x108", err)
	}
	if !strings.Contains(err.Error(), "erase the flash") || !strings.Contains(err.Error(), "flash write error") {
		t.Errorf("message %q lacks the step or the reason", err)
	}
	if f.hardReset {
		t.Error("restarted the chip after a failure")
	}
}

func TestWriteGivesUpOnAPersistentBlockError(t *testing.T) {
	f := newFakeROM(t)
	f.failOn[opDeflData] = 0x07
	_, err := write(f, testImage(3000), 0, Options{})
	var status *StatusError
	if !errors.As(err, &status) || status.Code != 0x107 {
		t.Fatalf("got %v", err)
	}
	if n := bytes.Count(f.commands, []byte{opDeflData}); n != writeAttempts {
		t.Errorf("%d tries, want %d", n, writeAttempts)
	}
}

func TestWriteMD5Mismatch(t *testing.T) {
	f := newFakeROM(t)
	f.corruptMD5 = true
	_, err := write(f, testImage(3000), 0, Options{})
	if !errors.Is(err, ErrVerify) {
		t.Fatalf("got %v, want ErrVerify", err)
	}
	if f.hardReset {
		t.Error("restarted into firmware that failed its check")
	}
}

func TestWriteTimeout(t *testing.T) {
	f := newFakeROM(t)
	f.silent = true
	_, err := write(f, testImage(3000), 0, Options{})
	if !errors.Is(err, ErrNoBootloader) || !strings.Contains(err.Error(), "no response") {
		t.Fatalf("got %v", err)
	}
	if f.resets != connectAttempts {
		t.Errorf("%d resets, want %d", f.resets, connectAttempts)
	}
}

func TestWriteTimeoutMidTransfer(t *testing.T) {
	f := newFakeROM(t)
	l := newLoader(context.Background(), f, noSleep)
	if err := l.connect(); err != nil {
		t.Fatal(err)
	}
	f.silent = true
	_, err := l.readReg(chipMagicReg)
	if !errors.Is(err, ErrNoResponse) {
		t.Fatalf("got %v, want ErrNoResponse", err)
	}
}

func TestWriteRefusesOtherChips(t *testing.T) {
	f := newFakeROM(t)
	f.magic = 0x000007c6 // an ESP32-S2
	_, err := write(f, testImage(3000), 0, Options{})
	if !errors.Is(err, ErrWrongChip) {
		t.Fatalf("got %v, want ErrWrongChip", err)
	}
	if slices.Contains(f.commands, opDeflBegin) {
		t.Error("erased the flash of the wrong chip")
	}
}

func TestWriteFallsBackWhenTheChipIgnoresTheBaudChange(t *testing.T) {
	f := newFakeROM(t)
	f.ignoreBaud = true
	image := testImage(20000)
	res, err := write(f, image, 0, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Baud != RomBaud || f.portBaud != RomBaud {
		t.Errorf("baud %d, port %d: want the ROM's speed", res.Baud, f.portBaud)
	}
	if !bytes.Equal(f.flash[:len(image)], image) {
		t.Error("flash differs")
	}
}

func TestWriteFallsBackWhenTheBridgeCannotKeepUp(t *testing.T) {
	f := newFakeROM(t)
	f.bridgeMaxBaud = 460800 // the chip follows to 921600, the bridge does not
	image := testImage(20000)
	res, err := write(f, image, 0, Options{Baud: 921600})
	if err != nil {
		t.Fatal(err)
	}
	if res.Baud != RomBaud || f.resets != 2 {
		t.Errorf("baud %d after %d resets, want the ROM's speed after 2", res.Baud, f.resets)
	}
	if !bytes.Equal(f.flash[:len(image)], image) {
		t.Error("flash differs")
	}

	f = newFakeROM(t)
	f.bridgeMaxBaud = 460800
	if res, err = write(f, image, 0, Options{}); err != nil || res.Baud != DefaultBaud {
		t.Errorf("default speed: baud %d, %v", res.Baud, err)
	}
}

func TestWriteRejectsBadArguments(t *testing.T) {
	f := newFakeROM(t)
	for name, tc := range map[string]struct {
		image  []byte
		offset uint32
	}{
		"empty":     {nil, 0},
		"unaligned": {testImage(10), 0x123},
		"too big":   {testImage(10), FlashSize - 4},
	} {
		if _, err := write(f, tc.image, tc.offset, Options{}); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if len(f.commands) != 0 || f.resets != 0 {
		t.Error("touched the chip before checking the arguments")
	}
}

func TestWriteStopsWhenCancelled(t *testing.T) {
	f := newFakeROM(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Write(ctx, f, testImage(3000), 0, Options{sleep: noSleep})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

func TestSlipRoundTrip(t *testing.T) {
	packet := []byte{0, slipEnd, 1, slipEsc, 2, slipEscEnd, slipEscEsc, 0xff}
	encoded := slipEncode(packet)
	if encoded[0] != slipEnd || encoded[len(encoded)-1] != slipEnd || bytes.Contains(encoded[1:len(encoded)-1], []byte{slipEnd}) {
		t.Fatalf("framing: % x", encoded)
	}
	f := newFakeROM(t)
	f.out = append([]byte("boot log\r\n"), encoded...)
	r := frameReader{port: f}
	got, err := r.next()
	if err != nil || !bytes.Equal(got, packet) {
		t.Errorf("got % x, %v", got, err)
	}
	if _, err := r.next(); !errors.Is(err, ErrNoResponse) {
		t.Errorf("after the frame: %v", err)
	}
}

func TestSlipRejectsBadEscape(t *testing.T) {
	f := newFakeROM(t)
	f.out = []byte{slipEnd, 1, slipEsc, 0x42, 3, slipEnd}
	r := frameReader{port: f}
	if _, err := r.next(); err == nil || !strings.Contains(err.Error(), "escape") {
		t.Errorf("got %v", err)
	}
}

func TestChecksum(t *testing.T) {
	if got := checksum(nil); got != 0xef {
		t.Errorf("empty: 0x%x", got)
	}
	if got := checksum([]byte{0x01, 0x02, 0x04}); got != 0xef^0x07 {
		t.Errorf("0x%x", got)
	}
}

func TestStatusErrorMessage(t *testing.T) {
	msg := (&StatusError{"write to the flash", 0x107}).Error()
	if !strings.Contains(msg, "write to the flash") || !strings.Contains(msg, "checksum") {
		t.Error(msg)
	}
	if msg := (&StatusError{"x", 0x1ff}).Error(); !strings.Contains(msg, "unknown") {
		t.Error(msg)
	}
}
